package webhook

import (
	"fmt"

	kubevirtctl "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type usbDeviceClaimValidator struct {
	types.DefaultValidator

	vmCache kubevirtctl.VirtualMachineCache
}

func (udc *usbDeviceClaimValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"usbdeviceclaims"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.USBDeviceClaim{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Update,
			admissionregv1.Delete,
		},
	}
}

func NewUSBDeviceClaimValidator(vmCache kubevirtctl.VirtualMachineCache) types.Validator {
	return &usbDeviceClaimValidator{
		vmCache: vmCache,
	}
}

func (udc *usbDeviceClaimValidator) Delete(_ *types.Request, oldObj runtime.Object) error {
	usbClaimObj := oldObj.(*devicesv1beta1.USBDeviceClaim)
	vms, err := udc.vmCache.GetByIndex(VMByUSBDeviceClaim, usbClaimObj.Name)
	if err != nil {
		return err
	}
	if len(vms) > 0 {
		err := fmt.Errorf("usbdeviceclaim %s is still in use by vm %s/%s", usbClaimObj.Name, vms[0].Name, vms[0].Namespace)
		logrus.Errorf(err.Error())
		return err
	}

	return nil
}

func (udc *usbDeviceClaimValidator) Update(_ *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldUsbClaimObj := oldObj.(*devicesv1beta1.USBDeviceClaim)
	newUsbClaimObj := newObj.(*devicesv1beta1.USBDeviceClaim)

	if oldUsbClaimObj.Spec.UserName != newUsbClaimObj.Spec.UserName {
		err := fmt.Errorf("usbdeviceclaim %s username is immutable", newUsbClaimObj.Name)
		logrus.Errorf(err.Error())
		return err
	}

	return nil
}
