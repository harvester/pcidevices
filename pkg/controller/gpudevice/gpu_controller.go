package gpudevice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/executor"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

const (
	sriovManageCommand = "/usr/lib/nvidia/sriov-manage"
)

type Handler struct {
	ctx                 context.Context
	nodeName            string
	sriovGPUCache       ctl.SRIOVGPUDeviceCache
	vGPUCache           ctl.VGPUDeviceCache
	sriovGPUClient      ctl.SRIOVGPUDeviceClient
	vGPUController      ctl.VGPUDeviceController
	vGPUClient          ctl.VGPUDeviceClient
	pciDeviceClaimCache ctl.PCIDeviceClaimCache
	executor            executor.Executor
	options             []nvpci.Option
	vGPUDevicePlugins   map[string]*deviceplugins.VGPUDevicePlugin
	virtClient          kubecli.KubevirtClient
	cfg                 *rest.Config
}

func NewHandler(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController, pciDeviceClaim ctl.PCIDeviceClaimController, virtClient kubecli.KubevirtClient, options []nvpci.Option, cfg *rest.Config) (*Handler, error) {
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)

	// initial a default local executor.
	// the pod handler overrides this with the remote executor when a driver pod is found and resets it back
	// to local executor when pod is removed
	commandExecutor := executor.NewLocalExecutor(os.Environ())

	return &Handler{
		ctx:                 ctx,
		sriovGPUCache:       sriovGPUController.Cache(),
		sriovGPUClient:      sriovGPUController,
		vGPUCache:           vGPUController.Cache(),
		vGPUClient:          vGPUController,
		pciDeviceClaimCache: pciDeviceClaim.Cache(),
		executor:            commandExecutor,
		nodeName:            nodeName,
		options:             options,
		vGPUDevicePlugins:   make(map[string]*deviceplugins.VGPUDevicePlugin),
		virtClient:          virtClient,
		vGPUController:      vGPUController,
		cfg:                 cfg,
	}, nil
}

// Register setups up handlers for SRIOVGPUDevices and VGPUDevices
func Register(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController, pciDeviceClaimController ctl.PCIDeviceClaimController, podController ctlcorev1.PodController, cfg *rest.Config) error {
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		return err
	}
	h, err := NewHandler(ctx, sriovGPUController, vGPUController, pciDeviceClaimController, virtClient, nil, cfg)
	if err != nil {
		return err
	}
	sriovGPUController.OnChange(ctx, "on-gpu-change", h.OnGPUChange)
	vGPUController.OnChange(ctx, "on-vgpu-change", h.OnVGPUChange)
	vGPUController.OnChange(ctx, "update-plugins", h.reconcileEnabledVGPUPlugins)
	podController.OnChange(ctx, "watch-driver-pods", h.setupRemoteExecutor)
	return nil
}

// OnGPUChange performs enable/disable operations if needed
func (h *Handler) OnGPUChange(_ string, gpu *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	if gpu == nil || gpu.DeletionTimestamp != nil || gpu.Spec.NodeName != h.nodeName {
		return gpu, nil
	}

	enabled, gpuStatus, err := gpuhelper.GenerateGPUStatus(filepath.Join(v1beta1.SysDevRoot, gpu.Spec.Address), gpu.Spec.NodeName)
	if err != nil {
		return gpu, fmt.Errorf("error generating status for SRIOVGPUDevice %s: %v", gpu.Name, err)
	}

	// perform enable/disable operation as needed
	if gpu.Spec.Enabled != enabled {
		logrus.Debugf("performing gpu management for %s", gpu.Name)
		return h.manageGPU(gpu)
	}

	if !reflect.DeepEqual(gpu.Status, gpuStatus) {
		logrus.Debugf("updating gpu status for %s:", gpu.Name)
		gpu.Status = *gpuStatus
		return h.sriovGPUClient.UpdateStatus(gpu)
	}
	return nil, nil
}

// SetupSRIOVGPUDevices is called by the node controller to reconcile objects on startup and predefined intervals
func (h *Handler) SetupSRIOVGPUDevices() error {
	sriovGPUDevices, err := gpuhelper.IdentifySRIOVGPU(h.options, h.nodeName)
	if err != nil {
		return err
	}
	return h.reconcileSRIOVGPUSetup(sriovGPUDevices)
}

// reconcileSRIOVGPUSetup runs the core logic to reconcile the k8s view of node with actual state on the node
func (h *Handler) reconcileSRIOVGPUSetup(sriovGPUDevices []*v1beta1.SRIOVGPUDevice) error {
	// create missing SRIOVGPUdevices, skipping GPU's which are already passed through as PCIDevices
	for _, v := range sriovGPUDevices {
		// if pcideviceclaim already exists for SRIOVGPU, then likely this GPU is already passed through
		// skip creation of SriovGPUDevice object until PCIDeviceClaim exists
		existingClaim, err := h.pciDeviceClaimCache.Get(v.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error looking up pcideviceclaim for sriovGPUDevice %s: %v", v.Name, err)
		}
		if existingClaim != nil {
			// pciDeviceClaim exists skipping
			logrus.Debugf("skipping creation of vGPUDevice %s as PCIDeviceClaim exists", existingClaim.Name)
			continue
		}

		if err := h.createOrUpdateSRIOVGPUDevice(v); err != nil {
			return err
		}
	}
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}

	existingGPUs, err := h.sriovGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return err
	}

	for _, v := range existingGPUs {
		if !containsGPUDevices(v, sriovGPUDevices) {
			if err := h.sriovGPUClient.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("error deleting non existent GPU device %s: %v", v.Name, err)
			}
		}
	}

	return nil
}

// createOrUpdateSRIOVGPUDevice will check and create GPU if one doesnt exist. If one is found it will perform an update if needed
func (h *Handler) createOrUpdateSRIOVGPUDevice(gpu *v1beta1.SRIOVGPUDevice) error {
	_, err := h.sriovGPUCache.Get(gpu.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := h.sriovGPUClient.Create(gpu)
			return createErr
		}
	}

	return err
}

// containsGPUDevices checks if gpu exists in list of devices
func containsGPUDevices(gpu *v1beta1.SRIOVGPUDevice, gpuList []*v1beta1.SRIOVGPUDevice) bool {
	for _, v := range gpuList {
		if v.Name == gpu.Name {
			return true
		}
	}
	return false
}

// manageGPU performs sriovmanage on the appropriate GPU
func (h *Handler) manageGPU(gpu *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	var args []string
	if gpu.Spec.Enabled {
		args = append(args, "-e", gpu.Spec.Address)
	} else {
		args = append(args, "-d", gpu.Spec.Address)
	}

	output, err := h.executor.CheckReady()
	if err != nil {
		logrus.Error(string(output))
		return gpu, fmt.Errorf("error during readiness check: %v", err)
	}

	output, err = h.executor.Run(sriovManageCommand, args)
	if err != nil {
		logrus.Error(string(output))
		return gpu, fmt.Errorf("error performing sriovmanage operation: %v", err)
	}
	logrus.Debugf("sriov-manage output: %s", string(output))
	_, gpuStatus, err := gpuhelper.GenerateGPUStatus(filepath.Join(v1beta1.SysDevRoot, gpu.Spec.Address), gpu.Spec.NodeName)
	if err != nil {
		return gpu, err
	}
	gpu.Status = *gpuStatus
	return h.sriovGPUClient.UpdateStatus(gpu)
}

// setupRemoteExecutor watches for NVIDIA driver pods and switches the controller to use
// the remote executor logic. This ensures that all controllers can boot up successfully
// even when SRIOV GPU capability is not needed by end user
func (h *Handler) setupRemoteExecutor(_ string, pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil || pod.Namespace != v1beta1.DefaultNamespace || isNotDriverPod(pod) || pod.Spec.NodeName != h.nodeName {
		return pod, nil
	}
	var newExecutor executor.Executor
	var err error
	if pod.DeletionTimestamp.IsZero() {
		// pod found, setup remote executor
		logrus.Debugf("found pod %s on node %s", pod.Name, h.nodeName)
		newExecutor, err = executor.NewRemoteCommandExecutor(h.ctx, h.cfg, pod)
		if err != nil {
			return pod, err
		}
	} else {
		// reset to default local executor
		newExecutor = executor.NewLocalExecutor(os.Environ())
	}

	h.executor = newExecutor
	return pod, nil
}

func isNotDriverPod(pod *corev1.Pod) bool {
	if pod.Labels == nil {
		return true
	}

	elements := strings.Split(v1beta1.NvidiaDriverLabel, "=")

	if val, ok := pod.Labels[elements[0]]; ok && val == elements[1] {
		return false
	}

	return true
}
