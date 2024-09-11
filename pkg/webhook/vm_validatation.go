package webhook

import (
	"fmt"

	"github.com/harvester/harvester/pkg/webhook/types"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type vmDeviceHostValidator struct {
	types.DefaultValidator

	usbCache v1beta1.USBDeviceCache
	pciCache v1beta1.PCIDeviceCache
}

func (vmValidator *vmDeviceHostValidator) Resource() types.Resource {
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

func NewDeviceHostValidation(usbCache v1beta1.USBDeviceCache, pciCache v1beta1.PCIDeviceCache) types.Validator {
	return &vmDeviceHostValidator{
		usbCache: usbCache,
		pciCache: pciCache,
	}
}

func (vmValidator *vmDeviceHostValidator) Create(_ *types.Request, newObj runtime.Object) error {
	vmObject := newObj.(*kubevirtv1.VirtualMachine)

	if len(vmObject.Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		return nil
	}

	if err := vmValidator.validateDevicesFromSameNodes(vmObject); err != nil {
		return err
	}

	return nil
}

func (vmValidator *vmDeviceHostValidator) Update(_ *types.Request, _ runtime.Object, newObj runtime.Object) error {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)

	if len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		return nil
	}

	if err := vmValidator.validateDevicesFromSameNodes(vmObj); err != nil {
		return err
	}

	return nil
}

func (vmValidator *vmDeviceHostValidator) validateDevicesFromSameNodes(vmObj *kubevirtv1.VirtualMachine) error {
	var nodeName string

	for number, device := range vmObj.Spec.Template.Spec.Domain.Devices.HostDevices {
		if err := vmValidator.validateDevice(device, &nodeName, number, vmObj.Name); err != nil {
			return err
		}
	}

	return nil
}

func (vmValidator *vmDeviceHostValidator) validateDevice(device kubevirtv1.HostDevice, nodeName *string, number int, vmName string) error {
	errorMsgFormat := "hostDevices[].name %s/%s is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices %s"

	usb, err := vmValidator.usbCache.Get(device.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if *nodeName == "" && usb != nil {
		if usb.Status.ResourceName != device.DeviceName {
			return fmt.Errorf("hostDevices[%d].DeviceName %s is not found in USBDevice", number, device.DeviceName)
		}
		*nodeName = usb.Status.NodeName
		return nil
	}

	pci, err := vmValidator.pciCache.Get(device.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if *nodeName == "" && pci != nil {
		if pci.Status.ResourceName != device.DeviceName {
			return fmt.Errorf("hostDevices[%d].DeviceName %s is not found in PCIDevice", number, device.DeviceName)
		}
		*nodeName = pci.Status.NodeName
		return nil
	}

	if pci != nil && pci.Status.NodeName != *nodeName {
		return fmt.Errorf(errorMsgFormat, "pcidevice", pci.Name, vmName)
	}

	if usb != nil && usb.Status.NodeName != *nodeName {
		return fmt.Errorf(errorMsgFormat, "usbdevice", usb.Name, vmName)
	}

	return fmt.Errorf("hostDevices[%d].name %s is not found in USBDevice or PCIDevice", number, device.Name)
}
