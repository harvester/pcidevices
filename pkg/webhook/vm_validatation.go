package webhook

import (
	"fmt"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/harvester/pkg/webhook/types"

	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type vmDeviceHostValidator struct {
	types.DefaultValidator

	usbCache  v1beta1.USBDeviceCache
	pciCache  v1beta1.PCIDeviceCache
	vgpuCache v1beta1.VGPUDeviceCache
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

func NewDeviceHostValidation(usbCache v1beta1.USBDeviceCache, pciCache v1beta1.PCIDeviceCache, vgpuCache v1beta1.VGPUDeviceCache) types.Validator {
	return &vmDeviceHostValidator{
		usbCache:  usbCache,
		pciCache:  pciCache,
		vgpuCache: vgpuCache,
	}
}

func (vmValidator *vmDeviceHostValidator) Create(_ *types.Request, newObj runtime.Object) error {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)
	return vmValidator.validateDevices(vmObj)
}

func (vmValidator *vmDeviceHostValidator) Update(_ *types.Request, _ runtime.Object, newObj runtime.Object) error {
	vmObj := newObj.(*kubevirtv1.VirtualMachine)
	return vmValidator.validateDevices(vmObj)
}

func (vmValidator *vmDeviceHostValidator) validateDevices(vmObj *kubevirtv1.VirtualMachine) error {
	if len(vmObj.Spec.Template.Spec.Domain.Devices.HostDevices) != 0 {
		if err := vmValidator.validateHostDevices(vmObj); err != nil {
			return err
		}
		if err := vmValidator.validateDevicesFromSameNodes(vmObj); err != nil {
			return err
		}
	}

	if len(vmObj.Spec.Template.Spec.Domain.Devices.GPUs) != 0 {
		if err := vmValidator.validateGPUs(vmObj); err != nil {
			return err
		}
	}

	return nil
}

func (vmValidator *vmDeviceHostValidator) validateDevicesFromSameNodes(vmObj *kubevirtv1.VirtualMachine) error {

	var nodeName string
	errorMsgFormat := "device %s/%s is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices %s"

	for _, device := range vmObj.Spec.Template.Spec.Domain.Devices.HostDevices {
		usb, err := vmValidator.usbCache.Get(device.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			usb = nil
		}

		if nodeName == "" && usb != nil {
			nodeName = usb.Status.NodeName
			continue
		}

		pci, err := vmValidator.pciCache.Get(device.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			pci = nil
		}

		if nodeName == "" && pci != nil {
			nodeName = pci.Status.NodeName
			continue
		}

		if pci != nil && pci.Status.NodeName != nodeName {
			return fmt.Errorf(errorMsgFormat, "pcidevice", pci.Name, vmObj.Name)
		}

		if usb != nil && usb.Status.NodeName != nodeName {
			return fmt.Errorf(errorMsgFormat, "usbdevice", usb.Name, vmObj.Name)
		}
	}

	return nil
}

func (vmValidator *vmDeviceHostValidator) validateHostDevices(vmObj *kubevirtv1.VirtualMachine) error {
	for _, hostDevice := range vmObj.Spec.Template.Spec.Domain.Devices.HostDevices {
		var (
			foundInPCI, foundInUSB bool
			err                    error
		)

		if foundInPCI, err = vmValidator.validatePCIDevice(hostDevice.DeviceName); err != nil {
			return err
		}

		if foundInUSB, err = vmValidator.validateUSBDevice(hostDevice.DeviceName); err != nil {
			return err
		}

		if !foundInPCI && !foundInUSB {
			return fmt.Errorf("hostdevice %s: resource name %s not found in pcidevice and usbdevice cache", hostDevice.Name, hostDevice.DeviceName)
		}
	}
	return nil
}

func (vmValidator *vmDeviceHostValidator) validatePCIDevice(resourceName string) (found bool, err error) {
	pciDeviceObjs, err := vmValidator.pciCache.GetByIndex(PCIDeviceByResourceName, resourceName)
	if err != nil {
		return false, fmt.Errorf("error looking up pcidevice %s from cache: %v", resourceName, err)
	}

	if len(pciDeviceObjs) == 0 {
		return false, nil
	}

	return true, nil
}

func (vmValidator *vmDeviceHostValidator) validateUSBDevice(resourceName string) (found bool, err error) {
	usbDeviceObjs, err := vmValidator.usbCache.GetByIndex(USBDeviceByResourceName, resourceName)
	if err != nil {
		return false, fmt.Errorf("error looking up usbdevice %s from cache: %v", resourceName, err)
	}

	if len(usbDeviceObjs) == 0 {
		return false, nil
	}

	return true, nil
}

func (vmValidator *vmDeviceHostValidator) validateVGPUDevice(resourceName string) (found bool, err error) {
	vGPUDeviceObjs, err := vmValidator.vgpuCache.GetByIndex(vGPUDeviceByResourceName, resourceName)
	if err != nil {
		return false, fmt.Errorf("error looking up vGPU device %s from cache: %v", resourceName, err)
	}

	if len(vGPUDeviceObjs) == 0 {
		return false, nil
	}

	return true, nil
}

func (vmValidator *vmDeviceHostValidator) validateGPUs(vmObj *kubevirtv1.VirtualMachine) error {
	for _, gpu := range vmObj.Spec.Template.Spec.Domain.Devices.GPUs {
		foundInPCI, err := vmValidator.validateVGPUDevice(gpu.DeviceName)

		if err != nil {
			return err
		}

		if !foundInPCI {
			return fmt.Errorf("gpu device %s: resource name %s not found in pcidevice cache", gpu.Name, gpu.DeviceName)
		}
	}

	return nil
}
