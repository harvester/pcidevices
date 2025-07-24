package virtualmachine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"

	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt/pkg/util"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/config"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctldevicesv1beta1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/pcidevices/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/pcidevices/pkg/util/executor"
)

const (
	kubevirtVMLabelKey = "vm.kubevirt.io/name"
)

type Handler struct {
	ctx            context.Context
	vmCache        ctlkubevirtv1.VirtualMachineCache
	vmClient       ctlkubevirtv1.VirtualMachineClient
	vmi            ctlkubevirtv1.VirtualMachineInstanceController
	pod            ctlcorev1.PodController
	vgpuCache      ctldevicesv1beta1.VGPUDeviceCache
	pciDeviceCache ctldevicesv1beta1.PCIDeviceCache
	config         *rest.Config
	nodeName       string
}

func Register(ctx context.Context, management *config.FactoryManager) error {
	vmCache := management.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmClient := management.KubevirtFactory.Kubevirt().V1().VirtualMachine()
	vmi := management.KubevirtFactory.Kubevirt().V1().VirtualMachineInstance()
	pod := management.CoreFactory.Core().V1().Pod()
	vgpuCache := management.DeviceFactory.Devices().V1beta1().VGPUDevice().Cache()
	pciDeviceCache := management.DeviceFactory.Devices().V1beta1().PCIDevice().Cache()
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)
	h := Handler{
		ctx:            ctx,
		vmCache:        vmCache,
		vmClient:       vmClient,
		vmi:            vmi,
		pod:            pod,
		vgpuCache:      vgpuCache,
		pciDeviceCache: pciDeviceCache,
		config:         management.Cfg,
		nodeName:       nodeName,
	}
	vmi.OnChange(ctx, "virtual-machine-instance-handler", h.OnVMIChange)
	vmi.OnRemove(ctx, "virtual-machine-deletion", h.OnVMIDeletion)
	return nil
}

// OnVMIChange attempts to reconcile devices allocated to launcher pod associated with a running VMI
// and annotate the VM object with device annotations
func (h *Handler) OnVMIChange(name string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil || vmi.DeletionTimestamp != nil {
		logrus.WithFields(logrus.Fields{
			"name": name,
		}).Debug("skipping object, either does not exist or marked for deletion")
		return vmi, nil
	}

	logrus.WithFields(logrus.Fields{
		"name":      vmi.Name,
		"namespace": vmi.Namespace,
	}).Debug("reconcilling vmi device allocation")

	if vmi.Status.Phase != kubevirtv1.Running {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
		}).Debug("skipping vmi as it is not running")
		return vmi, nil
	}

	if len(vmi.Spec.Domain.Devices.HostDevices) == 0 && len(vmi.Spec.Domain.Devices.GPUs) == 0 {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
		}).Debug("skipping vmi as it does not request any host devices or gpus")
		return vmi, h.checkAndClearDeviceAllocation(vmi)
	}

	if vmi.Status.NodeName != h.nodeName {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
			"hostname":  vmi.Status.NodeName,
		}).Debug("skipping vmi as it is not scheduled on current node")
		return vmi, nil
	}
	return vmi, h.trackDevices(vmi)
}

// trackDevices reconciles GPU and HostDevices info
func (h *Handler) trackDevices(vmi *kubevirtv1.VirtualMachineInstance) error {
	envMap, err := h.generatePodEnvMap(vmi)
	if err != nil {
		return err
	}
	return h.reconcileDeviceAllocationDetails(vmi, envMap)
}

// reconcileDeviceAllocationDetails will reconcile envMap into device allocation annotation on vmi
// has been split into its own method to simplify testing
func (h *Handler) reconcileDeviceAllocationDetails(vmi *kubevirtv1.VirtualMachineInstance, envMap map[string]string) error {
	var pciDeviceMap, vGPUMap map[string]string
	selector := map[string]string{
		"nodename": vmi.Status.NodeName,
	}
	if len(vmi.Spec.Domain.Devices.HostDevices) > 0 {
		deviceList, err := h.pciDeviceCache.List(labels.SelectorFromSet(selector))
		if err != nil {
			return fmt.Errorf("error listing pcidevices from cache: %v", err)
		}
		pciDeviceMap = buildPCIDeviceMap(deviceList)
	}
	if len(vmi.Spec.Domain.Devices.GPUs) > 0 {
		deviceList, err := h.vgpuCache.List(labels.SelectorFromSet(selector))
		if err != nil {
			return fmt.Errorf("error listing vgpudevices from cache: %v", err)
		}
		vGPUMap = buildVGPUMap(deviceList)
	}
	// map to hold device details

	hostDeviceMap := reconcilePCIDeviceDetails(vmi, envMap, pciDeviceMap)
	gpuMap := reconcileGPUDetails(vmi, envMap, vGPUMap)

	// generate allocation details
	deviceDetails := generateAllocationDetails(hostDeviceMap, gpuMap)
	deviceDetailsBytes, err := json.Marshal(deviceDetails)
	if err != nil {
		return fmt.Errorf("error marshalling deviceDetails: %v", err)
	}

	return h.reconcileVMResourceAllocationAnnotation(vmi, string(deviceDetailsBytes))
}

// findPodForVMI leverages the fact that each pod associated with a VMI a label vm.kubevirt.io/name: $vmName
// this makes it easier to find the correct pod
func (h *Handler) findPodForVMI(vmi *kubevirtv1.VirtualMachineInstance) (*corev1.Pod, error) {
	podList, err := h.pod.List(vmi.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kubevirtVMLabelKey, vmi.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %v", err)
	}

	// if more than 1 pod is returned make sure only 1 is running
	// if more than 1 is running then error out since there may be a migration
	// running and reconcile again
	var runningPod corev1.Pod

	var count int
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPod = pod // we copy pod in case there is only 1 running, avoids having to iterate again
			count++
		}
	}

	if count != 1 {
		return nil, fmt.Errorf("expected to find 1 pod, but found %d associated with vmi %s", count, vmi.Name)
	}

	return &runningPod, nil
}

func (h *Handler) getPodEnv(pod *corev1.Pod) ([]byte, error) {
	e, err := executor.NewRemoteCommandExecutor(h.ctx, h.config, pod)
	if err != nil {
		return nil, fmt.Errorf("error setting up remote executor for pod %s: %v", pod.Name, err)
	}
	return e.Run("env", []string{})
}

// convertEnvToMap converts env info to a map to make it easier to find device
// allocation details
func convertEnvToMap(podEnv []byte) (map[string]string, error) {
	var contents []string
	bufReader := bufio.NewReader(bytes.NewReader(podEnv))

	for {
		line, err := bufReader.ReadBytes('\n')
		if len(line) != 0 {
			contents = append(contents, strings.TrimSuffix(string(line), "\n"))
		}

		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}

	resultMap := make(map[string]string)
	for _, line := range contents {
		lineArr := strings.Split(line, "=")
		resultMap[lineArr[0]] = strings.Join(lineArr[1:], "=")
	}
	return resultMap, nil
}

// check and clear deviceAllocation annotations if needed
func (h *Handler) checkAndClearDeviceAllocation(vmi *kubevirtv1.VirtualMachineInstance) error {
	vmObj, err := h.vmCache.Get(vmi.Namespace, vmi.Name)
	if err != nil {
		return fmt.Errorf("error fetching vm %s from cache: %s", vmi.Name, err)
	}

	_, ok := vmObj.Annotations[v1beta1.DeviceAllocationKey]
	// no key, nothing is needed
	if !ok {
		return nil
	}
	delete(vmObj.Annotations, v1beta1.DeviceAllocationKey)
	_, err = h.vmClient.Update(vmObj)
	return err
}

func buildVGPUMap(vgpuDevices []*v1beta1.VGPUDevice) map[string]string {
	result := make(map[string]string)
	for _, vgpu := range vgpuDevices {
		if vgpu.Status.UUID != "" {
			result[vgpu.Status.UUID] = vgpu.Name
		}
	}
	return result
}

func buildPCIDeviceMap(pciDevices []*v1beta1.PCIDevice) map[string]string {
	result := make(map[string]string)
	for _, device := range pciDevices {
		result[device.Status.Address] = device.Name
	}
	return result
}

func generateAllocationDetails(hostDeviceMap, gpuMap map[string][]string) *v1beta1.AllocationDetails {
	resp := &v1beta1.AllocationDetails{}
	if len(hostDeviceMap) > 0 {
		hostDeviceMap = dedupDevices(hostDeviceMap)
		resp.HostDevices = hostDeviceMap
	}

	if len(gpuMap) > 0 {
		gpuMap = dedupDevices(gpuMap)
		resp.GPUs = gpuMap
	}
	return resp
}

// if multiple devices of same type are added to a VM, there may be duplicates in the deviceDetails values
// since env variable would have been looked up twice from envMap and added to deviceDetails
// so we dedup the unique device names
func dedupDevices(deviceMap map[string][]string) map[string][]string {
	for key, val := range deviceMap {
		deviceMap[key] = slices.Compact(val)
	}
	return deviceMap
}

// generatePodEnvMap attempts to find the pod associated with vmi, exec into pod to fetch `env` output
// and converts the same to the map, to allow the controller to identify device allocated to pod by kubelet
// which can differ from the name in the vmi devices spec, since allocation is only performed by resourceName
func (h *Handler) generatePodEnvMap(vmi *kubevirtv1.VirtualMachineInstance) (map[string]string, error) {
	logrus.WithFields(logrus.Fields{
		"name":      vmi.Name,
		"namespace": vmi.Namespace,
	}).Debug("looking up pod associated with vmi")

	pod, err := h.findPodForVMI(vmi)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      pod.Name,
		"namespace": pod.Namespace,
	}).Debug("looking up pod env")

	podEnv, err := h.getPodEnv(pod)
	if err != nil {
		return nil, err
	}

	envMap, err := convertEnvToMap(podEnv)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      vmi.Name,
		"namespace": vmi.Namespace,
	}).Debugf("found envMap: %v", envMap)
	return envMap, nil
}

func (h *Handler) reconcileVMResourceAllocationAnnotation(vmi *kubevirtv1.VirtualMachineInstance, deviceDetails string) error {
	logrus.WithFields(logrus.Fields{
		"name":      vmi.Name,
		"namespace": vmi.Namespace,
	}).Debugf("device allocation details: %s", deviceDetails)

	vmObj, err := h.vmCache.Get(vmi.Namespace, vmi.Name)
	if err != nil {
		return fmt.Errorf("error fetching vm %s from cache: %v", vmi.Name, err)
	}

	var currentAnnotationValue string
	// update device allocation details
	if vmObj.Annotations == nil {
		vmObj.Annotations = make(map[string]string)
	} else {
		currentAnnotationValue = vmObj.Annotations[v1beta1.DeviceAllocationKey]
	}

	if currentAnnotationValue != deviceDetails {
		vmObj.Annotations[v1beta1.DeviceAllocationKey] = deviceDetails
		_, err = h.vmClient.Update(vmObj)
	}
	return err
}

// OnVMIDeletion will update the VM spec with actual device allocation details
// this simplifies deletion of devices from the VM object without needing any changes
// it needs to be done during the VM shutdown avoid the object generation to differ between VM and VMI object
// and this avoids the generation warning being reported in the UI
func (h *Handler) OnVMIDeletion(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil || vmi.DeletionTimestamp == nil {
		return vmi, nil
	}

	// no host or GPU devices present so nothing needed to be done
	if len(vmi.Spec.Domain.Devices.GPUs) == 0 && len(vmi.Spec.Domain.Devices.HostDevices) == 0 {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
			"hostname":  vmi.Status.NodeName,
		}).Debug("skipping vmi as it has no hostdevices or GPUs")
		return vmi, nil
	}

	vmObj, err := h.vmCache.Get(vmi.Namespace, vmi.Name)
	if err != nil {
		return vmi, fmt.Errorf("error fetching vm object for vmi %s/%s: %v", vmi.Namespace, vmi.Name, err)
	}

	if vmi.Status.NodeName != h.nodeName {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
			"hostname":  vmi.Status.NodeName,
		}).Debug("skipping vmi as it is not scheduled on current node")
		return vmi, nil
	}

	val, ok := vmObj.Annotations[v1beta1.DeviceAllocationKey]
	if !ok {
		// no device allocation annotations, nothing to do
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
			"hostname":  vmi.Status.NodeName,
		}).Debug("skipping vmi as it has no device allocation annotation")
		return vmi, nil
	}

	allocationDetails := &v1beta1.AllocationDetails{}
	err = json.Unmarshal([]byte(val), allocationDetails)
	if err != nil {
		return vmi, fmt.Errorf("error unmarshalling allocation details annotation: %v", err)
	}

	vmObjCopy := vmObj.DeepCopy()
	if len(vmi.Spec.Domain.Devices.HostDevices) > 0 {
		patchHostDevices(vmObj, allocationDetails.HostDevices)
	}

	if len(vmi.Spec.Domain.Devices.GPUs) > 0 {
		patchGPUDevices(vmObj, allocationDetails.GPUs)
	}

	if !reflect.DeepEqual(vmObj.Spec.Template.Spec.Domain.Devices, vmObjCopy.Spec.Template.Spec.Domain.Devices) {
		logrus.WithFields(logrus.Fields{
			"name":      vmi.Name,
			"namespace": vmi.Namespace,
		}).Debugf("updating vm device allocation details: %v", vmObj.Spec.Template.Spec.Domain.Devices)
		_, err := h.vmClient.Update(vmObj)
		return vmi, err
	}
	return vmi, nil
}

func patchHostDevices(vm *kubevirtv1.VirtualMachine, deviceInfo map[string][]string) {
	for i, device := range vm.Spec.Template.Spec.Domain.Devices.HostDevices {
		actualDevices, ok := deviceInfo[device.DeviceName]
		if !ok {
			continue // should not ideally be possible but in case it does happen we ignore and continue
		}
		// pop first element of the actualDevices and set to deviceNames
		vm.Spec.Template.Spec.Domain.Devices.HostDevices[i].Name = actualDevices[0]
		actualDevices = actualDevices[1:]
		deviceInfo[device.DeviceName] = actualDevices // update map to ensure same name is not reused
	}
}

func patchGPUDevices(vm *kubevirtv1.VirtualMachine, deviceInfo map[string][]string) {
	for i, device := range vm.Spec.Template.Spec.Domain.Devices.GPUs {
		actualDevices, ok := deviceInfo[device.DeviceName]
		if !ok {
			continue // should not ideally be possible but in case it does happen we ignore and continue
		}
		// pop first element of the actualDevices and set to deviceNames
		vm.Spec.Template.Spec.Domain.Devices.GPUs[i].Name = actualDevices[0]
		actualDevices = actualDevices[1:]
		deviceInfo[device.DeviceName] = actualDevices // update map to ensure same name is not reused
	}
}

func reconcileGPUDetails(vmi *kubevirtv1.VirtualMachineInstance, envMap map[string]string, vGPUMap map[string]string) map[string][]string {
	gpuMap := make(map[string][]string)
	for _, device := range vmi.Spec.Domain.Devices.GPUs {
		val, ok := envMap[util.ResourceNameToEnvVar(deviceplugins.VGPUPrefix, device.DeviceName)]
		if ok {
			deviceInfo, deviceFound := gpuMap[device.DeviceName]
			// if there are multiple vGPU of same type then the environment variable
			// will contain details of all GPUs in the envMap
			// for example a VM with 2 vGPU of the same kind: MDEV_PCI_RESOURCE_NVIDIA_COM_NVIDIA_A2-4Q=e898f311-6b9e-46a2-b728-144d01af1a7c,95242535-e423-41a6-bb58-f28a44d68d66
			// as a result all GPUs will be added in the first pass of the key not being found
			// and we ignore subsequent lookups of the resource name key
			if !deviceFound {
				devices := strings.Split(val, ",")
				for _, v := range devices {
					deviceInfo = append(deviceInfo, vGPUMap[v])
				}
				gpuMap[device.DeviceName] = deviceInfo
			}
		}
	}
	return gpuMap
}

func reconcilePCIDeviceDetails(vmi *kubevirtv1.VirtualMachineInstance, envMap map[string]string, pciDeviceMap map[string]string) map[string][]string {
	hostDeviceMap := make(map[string][]string)
	for _, device := range vmi.Spec.Domain.Devices.HostDevices {
		val, ok := envMap[util.ResourceNameToEnvVar(deviceplugins.PCIResourcePrefix, device.DeviceName)]
		if ok {
			deviceInfo, deviceFound := hostDeviceMap[device.DeviceName]
			if !deviceFound {
				devices := strings.Split(val, ",")
				// currently our pcidevice plugin duplicates pci addresses
				// extra step needed to dedup addresses
				devices = slices.Compact(devices)
				for _, v := range devices {
					deviceInfo = append(deviceInfo, pciDeviceMap[v])
				}
				hostDeviceMap[device.DeviceName] = deviceInfo
			}

		}
	}
	return hostDeviceMap
}
