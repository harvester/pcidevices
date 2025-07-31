package gpudevice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/config"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/executor"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

const (
	sriovManageCommand = "/usr/lib/nvidia/sriov-manage"
)

type Handler struct {
	ctx                        context.Context
	nodeName                   string
	sriovGPUCache              ctl.SRIOVGPUDeviceCache
	vGPUCache                  ctl.VGPUDeviceCache
	sriovGPUClient             ctl.SRIOVGPUDeviceClient
	vGPUController             ctl.VGPUDeviceController
	vGPUClient                 ctl.VGPUDeviceClient
	pciDeviceClaimCache        ctl.PCIDeviceClaimCache
	migConfigurationCache      ctl.MigConfigurationCache
	migConfigurationController ctl.MigConfigurationController
	executor                   executor.Executor
	options                    []nvpci.Option
	vGPUDevicePlugins          map[string]*deviceplugins.VGPUDevicePlugin
	virtClient                 kubecli.KubevirtClient
	cfg                        *rest.Config
}

func NewHandler(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController, pciDeviceClaim ctl.PCIDeviceClaimController, migConfigurationController ctl.MigConfigurationController, virtClient kubecli.KubevirtClient, options []nvpci.Option, cfg *rest.Config) (*Handler, error) {
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)

	// initial a default local executor.
	// the pod handler overrides this with the remote executor when a driver pod is found and resets it back
	// to local executor when pod is removed
	commandExecutor := executor.NewLocalExecutor(os.Environ())

	return &Handler{
		ctx:                        ctx,
		sriovGPUCache:              sriovGPUController.Cache(),
		sriovGPUClient:             sriovGPUController,
		vGPUCache:                  vGPUController.Cache(),
		vGPUClient:                 vGPUController,
		pciDeviceClaimCache:        pciDeviceClaim.Cache(),
		migConfigurationCache:      migConfigurationController.Cache(),
		migConfigurationController: migConfigurationController,
		executor:                   commandExecutor,
		nodeName:                   nodeName,
		options:                    options,
		vGPUDevicePlugins:          make(map[string]*deviceplugins.VGPUDevicePlugin),
		virtClient:                 virtClient,
		vGPUController:             vGPUController,
		cfg:                        cfg,
	}, nil
}

// Register setups up handlers for SRIOVGPUDevices and VGPUDevices
func Register(ctx context.Context, management *config.FactoryManager) error {
	sriovGPUController := management.DeviceFactory.Devices().V1beta1().SRIOVGPUDevice()
	vGPUController := management.DeviceFactory.Devices().V1beta1().VGPUDevice()
	pciDeviceClaimController := management.DeviceFactory.Devices().V1beta1().PCIDeviceClaim()
	podController := management.CoreFactory.Core().V1().Pod()
	migConfigurationController := management.DeviceFactory.Devices().V1beta1().MigConfiguration()

	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		return err
	}
	h, err := NewHandler(ctx, sriovGPUController, vGPUController, pciDeviceClaimController, migConfigurationController, virtClient, nil, management.Cfg)
	if err != nil {
		return err
	}
	sriovGPUController.OnChange(ctx, "on-gpu-change", h.OnGPUChange)
	sriovGPUController.OnChange(ctx, "gpu-mig-reconcillation", h.reconcileMIGConfiguration)
	vGPUController.OnChange(ctx, "on-vgpu-change", h.OnVGPUChange)
	vGPUController.OnChange(ctx, "update-plugins", h.reconcileEnabledVGPUPlugins)
	podController.OnChange(ctx, "watch-driver-pods", h.setupRemoteExecutor)
	migConfigurationController.OnChange(ctx, "on-migconfiguration-change", h.OnMIGChange)
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
	return gpu, nil
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
		if err != nil {
			// due to wrangler bump to v3.1.0, when object is not found, the return is as an empty object and not nil along with the error. The following ensures
			// that we mark existingClaim to nil
			// to ensure no more changes are needed to processing logic
			if apierrors.IsNotFound(err) {
				existingClaim = nil
			} else {
				return fmt.Errorf("error looking up pcideviceclaim for sriovGPUDevice %s: %v", v.Name, err)
			}
		}

		//wrangler bump to v3.1.
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

// setupMigconfiguration will check if GPU suports MIG instances
// and accordingly creates a MIGConfigurationObject if one is not already present
func (h *Handler) setupMigConfiguration(gpu *v1beta1.SRIOVGPUDevice) error {
	// check if GPU supports MIG instances
	ok, err := gpuhelper.IsMigConfigurationNeeded(h.executor, gpu.Spec.Address)
	if err != nil {
		return fmt.Errorf("error checking if GPU supports MIG instances: %w", err)
	}

	// if GPU does not support MIG instances
	// then no MIGConfiguration object is created
	if !ok {
		logrus.Debugf("skipping GPU device %s, as it does not support MIG instances", gpu.Name)
		return nil
	}

	// enable MIGMode on device, this is idempotent so can be run multiple times
	if err := gpuhelper.EnableMIGMode(h.executor, gpu.Spec.Address); err != nil {
		return fmt.Errorf("error enable MIG Mode: %w", err)
	}

	// MIGConfiguration object has same name as GPU Name so we can use that to lookup existing device
	obj, err := h.migConfigurationCache.Get(gpu.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error looking up MIG Configuration: %w", err)
	}

	if obj != nil {
		// existing object found, nothing else is needed
		return nil
	}

	migConfiguration, err := gpuhelper.GenerateMIGConfiguration(h.executor, gpu.Spec.Address, gpu.Name)
	if err != nil {
		return fmt.Errorf("error generating MIG Configuration: %w", err)
	}

	migStatus, err := gpuhelper.GenerateMIGConfigurationStatus(h.executor, gpu.Spec.Address)
	if err != nil {
		return fmt.Errorf("error generating MIG configuration status: %w", err)
	}

	migConfiguration.Spec.NodeName = gpu.Spec.NodeName
	if instanceCount(migStatus) > 0 {
		migConfiguration.Spec.Enabled = true
	}

	migConfiguration.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: gpu.APIVersion,
			Kind:       gpu.Kind,
			Name:       gpu.Name,
			UID:        gpu.UID,
		},
	}
	// setup a new MIG configuration object
	migObject, err := h.migConfigurationController.Create(migConfiguration)
	if err != nil {
		return fmt.Errorf("error setting up MIG configuration: %w", err)
	}

	migObject.Status = *migStatus

	_, err = h.migConfigurationController.UpdateStatus(migObject)
	return err
}

func (h *Handler) reconcileMIGConfiguration(_ string, gpu *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	if gpu.Spec.Enabled {
		return gpu, h.setupMigConfiguration(gpu)
	}

	// trigger deletion of MIGConfiguration object
	_, err := h.migConfigurationCache.Get(gpu.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// object does not exist, no further action needed
			return gpu, nil
		}
		return gpu, err
	}

	return gpu, h.migConfigurationController.Delete(gpu.Name, &metav1.DeleteOptions{})
}
