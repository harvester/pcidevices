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
	defaultHostDevBasePath = "/spec/template/spec/domain/devices/hostDevices/-"
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

func (vm *vmPCIMutator) Create(request *types.Request, newObj runtime.Object) (types.PatchOps, error) {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)

	if len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		return nil, nil
	}

	return vm.generatePatch(vmObj)
}

func (vm *vmPCIMutator) Update(request *types.Request, oldObj runtime.Object, newObj runtime.Object) (types.PatchOps, error) {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)
	oldVMObj := newObj.(*kubevirtv1.VirtualMachine)

	if len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		return nil, nil
	}

	if reflect.DeepEqual(oldVMObj.Spec.Template.Spec.Domain.Devices.HostDevices, vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) {
		// no changes to host device, ignore request
		return nil, nil
	}

	return vm.generatePatch(vmObj)
}

// generatePatch is a common method used by create and update calls to generate a patch operation for VM
func (vm *vmPCIMutator) generatePatch(vmObj *kubevirtv1.VirtualMachine) (types.PatchOps, error) {
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
	if err == nil {
		logrus.Debugf("generated patch for vm %s in ns %s: %v", vmObj.Name, vmObj.Namespace, patch)
	}
	return patch, err
}

// generate patch for host devices in VM spec
func generatePatchFromDevices(devicesNeeded []pciDeviceWithOwners) (types.PatchOps, error) {
	var patchOps types.PatchOps
	for _, dev := range devicesNeeded {
		devPatch, err := generateDevicePatch(dev.device)
		if err != nil {
			return nil, fmt.Errorf("error generating patch for device %s: %v", dev.device.Name, err)
		}
		patchOps = append(patchOps, devPatch...)
	}
	return patchOps, nil
}

func generateDevicePatch(dev *devicesv1beta1.PCIDevice) (types.PatchOps, error) {
	var patchOps types.PatchOps
	hostDev := &kubevirtv1.HostDevice{
		Name:       dev.Name,
		DeviceName: dev.Status.ResourceName,
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
