package webhook

import (
	"fmt"
	"reflect"

	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	kubevirtv1 "kubevirt.io/api/core/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	defaultHostDevBase     = "/spec/template/spec/domain/devices/hostDevices"
	defaultHostDevBasePath = "/spec/template/spec/domain/devices/hostDevices/-"
	defaultGPUDevBasePath  = "/spec/template/spec/domain/devices/gpus"
)

func NewPCIVMMutator(deviceCache v1beta1.PCIDeviceCache, pciClaimCache v1beta1.PCIDeviceClaimCache, pciClaimClient v1beta1.PCIDeviceClaimClient) types.Mutator {
	return &vmPCIMutator{
		deviceCache:    deviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}
}

// vmPCIDeviceMutator injects additional pcidevice claim information into the VM based on iommu grouping of currently
// provided devices
type vmPCIMutator struct {
	types.DefaultMutator
	deviceCache    v1beta1.PCIDeviceCache
	pciClaimCache  v1beta1.PCIDeviceClaimCache
	pciClaimClient v1beta1.PCIDeviceClaimClient
}

// pciDeviceWithOwners is used to track the owner of a device along with owner.
// this is used to create additional pcidevice claims with the same username
type pciDeviceWithOwners struct {
	device *devicesv1beta1.PCIDevice
	owner  string
}

// Mutator is applied on create/update requests as pcidevices can be added during these two operations
func (vm *vmPCIMutator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"virtualmachines"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   kubevirtv1.SchemeGroupVersion.Group,
		APIVersion: kubevirtv1.SchemeGroupVersion.Version,
		ObjectType: &kubevirtv1.VirtualMachine{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Update,
		},
	}
}

func (vm *vmPCIMutator) Create(_ *types.Request, newObj runtime.Object) (types.PatchOps, error) {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)

	var patches types.PatchOps
	if len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) != 0 {
		hostPatches, err := vm.generateHostDevicesPatch(vmObj)
		if err != nil {
			return patches, fmt.Errorf("error generating hostdevices patch for vm %s/%s: %w", vmObj.Namespace, vmObj.Name, err)
		}
		patches = append(patches, hostPatches...)
	}

	if len(vmObj.Spec.Template.Spec.Domain.Devices.GPUs) != 0 {
		gpuPatches, err := convertGPUsToHostDevices(vmObj)
		if err != nil {
			return patches, fmt.Errorf("error generating gpu conversion patch for vm %s/%s: %w", vmObj.Namespace, vmObj.Name, err)
		}
		patches = append(patches, gpuPatches...)
	}
	return patches, nil
}

func (vm *vmPCIMutator) Update(_ *types.Request, _ runtime.Object, newObj runtime.Object) (types.PatchOps, error) {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)
	oldVMObj := newObj.(*kubevirtv1.VirtualMachine)

	var patches types.PatchOps
	if !reflect.DeepEqual(oldVMObj.Spec.Template.Spec.Domain.Devices.HostDevices, vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) {
		hostPatches, err := vm.generateHostDevicesPatch(vmObj)
		if err != nil {
			return patches, fmt.Errorf("error generating hostdevices patch for vm %s/%s: %w", vmObj.Namespace, vmObj.Name, err)
		}
		patches = append(patches, hostPatches...)
	}

	// if any GPUs exist in the updated vm, we try and convert them to host devices
	if len(vmObj.Spec.Template.Spec.Domain.Devices.GPUs) != 0 {
		gpuPatches, err := convertGPUsToHostDevices(vmObj)
		if err != nil {
			return patches, fmt.Errorf("error generating gpu conversion patch for vm %s/%s: %w", vmObj.Namespace, vmObj.Name, err)
		}
		patches = append(patches, gpuPatches...)
	}
	return patches, nil
}

// generatePatch is a common method used by create and update calls to generate a patch operation for VM
func (vm *vmPCIMutator) generateHostDevicesPatch(vmObj *kubevirtv1.VirtualMachine) (types.PatchOps, error) {
	pciDevicesInVM := make([]string, 0, len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices))
	var possiblePCIDeviceRequirement []pciDeviceWithOwners
	for _, v := range vmObj.Spec.Template.Spec.Domain.Devices.HostDevices {
		// ui sends name to be same as the pcidevice claim name, which in turn matches pcidevice
		// lookup is needed to query iommu group for said device
		pciDeviceObj, err := vm.deviceCache.Get(v.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil // pre 1.1.2 UI changes the device name did not match pcidevice. This avoids breaking
			}
			return nil, fmt.Errorf("error looking up pcidevice %s from cache: %v", v.Name, err)
		}

		pciDeviceClaimObj, err := vm.pciClaimCache.Get(v.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("error looking up pcidevice %s from cache: %v", v.Name, err)
		}

		pciDevices, err := vm.deviceCache.GetByIndex(IommuGroupByNode, fmt.Sprintf("%s-%s", pciDeviceObj.Status.NodeName, pciDeviceObj.Status.IOMMUGroup))
		if err != nil {
			logrus.Errorf("error looking up pcidevices for vm %s: %v", v.Name, err)
			return nil, fmt.Errorf("error lookup up pcidevices: %v", err)
		}
		pciDevicesInVM = append(pciDevicesInVM, v.Name)
		for _, v := range pciDevices {
			possiblePCIDeviceRequirement = append(possiblePCIDeviceRequirement, pciDeviceWithOwners{
				device: v,
				owner:  pciDeviceClaimObj.Spec.UserName,
			})
		}

	}

	devicesNeeded := identifyAdditionalPCIDevices(pciDevicesInVM, possiblePCIDeviceRequirement)
	if len(devicesNeeded) == 0 {
		return nil, nil // no further action needed as all devices are already present
	}

	for _, v := range devicesNeeded {
		if err := vm.findAndCreateClaim(v.device, v.owner); err != nil {
			return nil, fmt.Errorf("error during findAndCreateClaim: %v", err)
		}
	}
	patch, err := generatePatchFromDevices(devicesNeeded)
	if err != nil {
		return patch, err
	}

	vgpuPatch, err := convertGPUsToHostDevices(vmObj)
	if err != nil {
		return patch, fmt.Errorf("error converting GPUs to HostDevices: %w", err)
	}
	patch = append(patch, vgpuPatch...)
	logrus.Debugf("generated patch for vm %s in ns %s: %v", vmObj.Name, vmObj.Namespace, patch)
	return patch, err
}

// generate patch for host devices in VM spec
func generatePatchFromDevices(devicesNeeded []pciDeviceWithOwners) (types.PatchOps, error) {
	var patchOps types.PatchOps
	for _, dev := range devicesNeeded {
		devPatch, err := generateDevicePatch(dev.device.Name, dev.device.Status.ResourceName)
		if err != nil {
			return nil, fmt.Errorf("error generating patch for device %s: %v", dev.device.Name, err)
		}
		patchOps = append(patchOps, devPatch...)
	}
	return patchOps, nil
}

func generateDevicePatch(name, resourceName string) (types.PatchOps, error) {
	var patchOps types.PatchOps
	hostDev := &kubevirtv1.HostDevice{
		Name:       name,
		DeviceName: resourceName,
	}

	hostDevStr, err := json.Marshal(hostDev)
	if err != nil {
		return nil, fmt.Errorf("error marshalling host device into string :%v", err)
	}

	patchOps = append(patchOps, fmt.Sprintf(`{"op": "add", "path": "%s", "value": %s}`, defaultHostDevBasePath, hostDevStr))
	return patchOps, nil
}

func identifyAdditionalPCIDevices(pciDevicesInVM []string, possiblePCIDeviceRequirement []pciDeviceWithOwners) []pciDeviceWithOwners {
	var additionalDevicesNeeded []pciDeviceWithOwners
	for _, v := range possiblePCIDeviceRequirement {
		if !additionalDeviceAlreadyExists(pciDevicesInVM, v.device.Name) {
			additionalDevicesNeeded = append(additionalDevicesNeeded, v)
		}
	}
	return additionalDevicesNeeded
}

func (vm *vmPCIMutator) findAndCreateClaim(dev *devicesv1beta1.PCIDevice, owner string) error {
	_, err := vm.pciClaimCache.Get(dev.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			newClaim := generatePCIDeviceClaim(dev, owner)
			_, createErr := vm.pciClaimClient.Create(newClaim)
			return createErr
		}
		return err
	}

	// claim exists
	return nil
}

func generatePCIDeviceClaim(dev *devicesv1beta1.PCIDevice, owner string) *devicesv1beta1.PCIDeviceClaim {
	return &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: dev.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: dev.APIVersion,
					Kind:       dev.Kind,
					Name:       dev.Name,
					UID:        dev.UID,
				},
			},
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			NodeName: dev.Status.NodeName,
			Address:  dev.Status.Address,
			UserName: owner,
		},
	}
}

func additionalDeviceAlreadyExists(devList []string, dev string) bool {
	for _, v := range devList {
		if v == dev {
			return true
		}
	}

	return false
}

func convertGPUsToHostDevices(vm *kubevirtv1.VirtualMachine) (types.PatchOps, error) {
	var conversionPatch types.PatchOps
	if len(vm.Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		// no hostdevices present so need to initalise an empty array
		conversionPatch = append(conversionPatch, fmt.Sprintf(`{"op": "add", "path": "%s", "value": %v}`, defaultHostDevBase, "[]"))
	}
	for i, vgpu := range vm.Spec.Template.Spec.Domain.Devices.GPUs {
		deviceIndexPath := fmt.Sprintf("%s/%d", defaultGPUDevBasePath, i)
		// delete gpu device from section
		conversionPatch = append(conversionPatch, fmt.Sprintf(`{"op": "remove", "path": "%s"}`, deviceIndexPath))
		// DeviceName in vGPU or HostDevices maps to generated ResourceName advertised by the plugin
		devPatch, err := generateDevicePatch(vgpu.Name, vgpu.DeviceName)
		if err != nil {
			return conversionPatch, fmt.Errorf("error converting vGPU to hostdevice: %w", err)
		}
		conversionPatch = append(conversionPatch, devPatch...)
	}
	return conversionPatch, nil
}
