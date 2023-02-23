package webhook

import (
	"fmt"
	"strings"

	"github.com/harvester/harvester/pkg/webhook/types"
	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type sriovNetworkDeviceValidator struct {
	types.DefaultValidator
	claimCache v1beta1.PCIDeviceClaimCache
}

func NewSriovNetworkDeviceValidator(claimCache v1beta1.PCIDeviceClaimCache) types.Validator {
	return &sriovNetworkDeviceValidator{
		claimCache: claimCache,
	}
}

func (s *sriovNetworkDeviceValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"sriovnetworkdevices"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.SRIOVNetworkDevice{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Delete,
			admissionregv1.Update,
		},
	}
}

func (s *sriovNetworkDeviceValidator) Update(request *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldSriovDevice := oldObj.(*devicesv1beta1.SRIOVNetworkDevice)
	newSriovDevice := newObj.(*devicesv1beta1.SRIOVNetworkDevice)

	if oldSriovDevice.Spec.NumVFs == newSriovDevice.Spec.NumVFs {
		return nil
	}

	if newSriovDevice.Spec.NumVFs == 0 {
		return s.checkVFInUse(newSriovDevice)
	}

	return nil
}

func (s *sriovNetworkDeviceValidator) Delete(request *types.Request, oldObj runtime.Object) error {
	oldSriovDevice := oldObj.(*devicesv1beta1.SRIOVNetworkDevice)
	return s.checkVFInUse(oldSriovDevice)

}

func (s *sriovNetworkDeviceValidator) checkVFInUse(obj *devicesv1beta1.SRIOVNetworkDevice) error {
	claimsFound := make([]string, 0, len(obj.Status.VFPCIDevices))
	for _, v := range obj.Status.VFPCIDevices {
		vfObj, err := s.claimCache.Get(v)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Debugf("skipping vf pcidevice %s, as no claim exists for it", v)
				continue
			}
			return fmt.Errorf("error looking up pcideviceclaim: %v", err)
		}
		claimsFound = append(claimsFound, vfObj.Name)
	}

	// return pcideviceclaims in use
	if len(claimsFound) != 0 {
		logrus.Errorf("found pcideviceclaims: %s related to sriovnetworkdevice %s in use", strings.Join(claimsFound, ","), obj.Name)
		return fmt.Errorf("found pcideviceclaims: %s related to sriovnetworkdevice %s in use", strings.Join(claimsFound, ","), obj.Name)
	}

	return nil
}
