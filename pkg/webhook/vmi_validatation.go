package webhook

import (
	"fmt"

	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type vmiDeviceHostValidator struct {
	types.DefaultValidator

	usbCache v1beta1.USBDeviceCache
}

func (vmiValidator *vmiDeviceHostValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"virtualmachineinstances"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   kubevirtv1.SchemeGroupVersion.Group,
		APIVersion: kubevirtv1.SchemeGroupVersion.Version,
		ObjectType: &kubevirtv1.VirtualMachineInstance{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Update,
		},
	}
}

func NewDeviceHostValidation(usbCache v1beta1.USBDeviceCache) types.Validator {
	return &vmiDeviceHostValidator{
		usbCache: usbCache,
	}
}

func (vmiValidator *vmiDeviceHostValidator) Create(_ *types.Request, newObj runtime.Object) error {
	vmiObj := newObj.(*kubevirtv1.VirtualMachineInstance)

	if len(vmiObj.Spec.Domain.Devices.HostDevices) == 0 {
		return nil
	}

	if err := vmiValidator.checkUSBDevice(vmiObj); err != nil {
		return err
	}

	return nil
}

func (vmiValidator *vmiDeviceHostValidator) Update(_ *types.Request, _ runtime.Object, newObj runtime.Object) error {
	vmiObj := newObj.(*kubevirtv1.VirtualMachineInstance)

	if len(vmiObj.Spec.Domain.Devices.HostDevices) == 0 {
		return nil
	}

	if err := vmiValidator.checkUSBDevice(vmiObj); err != nil {
		return err
	}

	return nil
}

func (vmiValidator *vmiDeviceHostValidator) checkUSBDevice(vmiObj *kubevirtv1.VirtualMachineInstance) error {
	for _, device := range vmiObj.Spec.Domain.Devices.HostDevices {
		usb, err := vmiValidator.usbCache.Get(device.Name)

		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Device %s not found inside usbdevices", device.Name)
				continue
			}

			return err
		}

		vmiNodeName, ok := vmiObj.Labels["kubevirt.io/nodeName"]
		if !ok {
			continue
		}

		if usb.Status.NodeName != vmiNodeName {
			return fmt.Errorf("USB device %s is not on the same node as VirtualMachineInstance %s", usb.Name, vmiObj.Name)
		}
	}

	return nil
}
