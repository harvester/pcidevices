package webhook

import (
	"fmt"

	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/harvester/pkg/webhook/types"
	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type usbDeviceValidator struct {
	types.DefaultValidator
}

func (udc *usbDeviceValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"usbdevices"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.USBDevice{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Delete,
		},
	}
}

func NewUSBDeviceValidator() types.Validator {
	return &usbDeviceValidator{}
}

func (udc *usbDeviceValidator) Delete(_ *types.Request, oldObj runtime.Object) error {
	usbDevice := oldObj.(*devicesv1beta1.USBDevice)

	if usbDevice.Status.Enabled {
		err := fmt.Errorf("usbdevice %s is still in use", usbDevice.Name)
		logrus.Error(err)
		return err
	}

	return nil
}
