package webhook

import (
	"fmt"

	"github.com/harvester/harvester/pkg/webhook/types"
	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type usbDeviceValidator struct {
	types.DefaultValidator
	nodeCache ctlcorev1.NodeCache
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

func NewUSBDeviceValidator(nodeCache ctlcorev1.NodeCache) types.Validator {
	return &usbDeviceValidator{nodeCache: nodeCache}
}

func (udc *usbDeviceValidator) Delete(_ *types.Request, oldObj runtime.Object) error {
	usbDevice := oldObj.(*devicesv1beta1.USBDevice)

	ok, err := isNodeDeleted(udc.nodeCache, usbDevice.Status.NodeName)
	if err != nil {
		err := fmt.Errorf("error looking up node for usbdevice %s from node cache: %w", usbDevice.Name, err)
		logrus.Error(err)
		return err
	}

	// node related to usbdevice is no longer present, no need to validate further
	// allow deletion of object
	if ok {
		return nil
	}

	if usbDevice.Status.Enabled {
		err := fmt.Errorf("usbdevice %s is still in use", usbDevice.Name)
		logrus.Error(err)
		return err
	}

	return nil
}
