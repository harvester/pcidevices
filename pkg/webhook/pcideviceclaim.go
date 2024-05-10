package webhook

import (
	"fmt"

	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubevirtctl "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester/pkg/webhook/types"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type pciDeviceClaimValidator struct {
	types.DefaultValidator
	deviceCache   v1beta1.PCIDeviceCache
	kubevirtCache kubevirtctl.VirtualMachineCache
}

func NewPCIDeviceClaimValidator(deviceCache v1beta1.PCIDeviceCache, kubevirtCache kubevirtctl.VirtualMachineCache) types.Validator {
	return &pciDeviceClaimValidator{
		deviceCache:   deviceCache,
		kubevirtCache: kubevirtCache,
	}
}

func (pdc *pciDeviceClaimValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"pcideviceclaims"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.PCIDeviceClaim{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Delete,
		},
	}
}

func (pdc *pciDeviceClaimValidator) Create(_ *types.Request, newObj runtime.Object) error {
	pciClaimObj := newObj.(*devicesv1beta1.PCIDeviceClaim)
	pciDev, err := pdc.deviceCache.Get(pciClaimObj.Name)
	if err != nil {
		return err
	}

	if pciDev.Status.IOMMUGroup == "" {
		logrus.Errorf("pcidevice %s has no iommuGroup available", pciDev.Name)
		return fmt.Errorf("pcidevice %s has no iommuGroup available", pciDev.Name)
	}

	return nil
}

func (pdc *pciDeviceClaimValidator) Delete(_ *types.Request, oldObj runtime.Object) error {
	pciClaimObj := oldObj.(*devicesv1beta1.PCIDeviceClaim)
	vms, err := pdc.kubevirtCache.GetByIndex(VMByPCIDeviceClaim, pciClaimObj.Name)
	if err != nil {
		return err
	}

	if len(vms) != 0 {
		logrus.Errorf("pcideviceclaim %s is already in use with vm %s in namespace %s", pciClaimObj.Name, vms[0].Name, vms[0].Namespace)
		return fmt.Errorf("pcideviceclaim %s is already in use with vm %s in namespace %s", pciClaimObj.Name, vms[0].Name, vms[0].Namespace)
	}

	return nil
}
