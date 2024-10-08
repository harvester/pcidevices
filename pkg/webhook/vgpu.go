package webhook

import (
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubevirtctl "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester/pkg/webhook/types"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type vgpuValidator struct {
	types.DefaultValidator
	kubevirtCache kubevirtctl.VirtualMachineCache
}

func NewVGPUValidator(kubevirtCache kubevirtctl.VirtualMachineCache) types.Validator {
	return &vgpuValidator{
		kubevirtCache: kubevirtCache,
	}
}

func (v *vgpuValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"vgpudevices"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.VGPUDevice{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Update,
			admissionregv1.Delete,
		},
	}
}

func (v *vgpuValidator) Update(_ *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldVGPUObj := oldObj.(*devicesv1beta1.VGPUDevice)
	newVGPUObj := newObj.(*devicesv1beta1.VGPUDevice)

	if reflect.DeepEqual(oldVGPUObj.Spec, newVGPUObj.Spec) {
		return nil
	}

	// vGPU was disabled, check if the device is in use with a vm
	if oldVGPUObj.Spec.Enabled && !newVGPUObj.Spec.Enabled {
		return checkVGPUUsage(v.kubevirtCache, newVGPUObj.Name)
	}

	// vGPU was enabled, run some basic sanity checks on request
	if !oldVGPUObj.Spec.Enabled && newVGPUObj.Spec.Enabled {
		return validateVGPUProfiles(oldVGPUObj, newVGPUObj)
	}
	return nil
}

func (v *vgpuValidator) Delete(_ *types.Request, obj runtime.Object) error {
	vGPUObj := obj.(*devicesv1beta1.VGPUDevice)
	return checkVGPUUsage(v.kubevirtCache, vGPUObj.Name)
}

func checkVGPUUsage(kc kubevirtctl.VirtualMachineCache, deviceName string) error {
	objs, err := kc.GetByIndex(VMByVGPU, deviceName)
	if err != nil {
		logrus.Errorf("error fetching VMs from cache: %v", err)
		return err
	}

	if len(objs) > 0 {
		return fmt.Errorf("device %s is in use with VM %s in namespace %s", deviceName, objs[0].Name, objs[0].Namespace)
	}

	return nil
}

func validateVGPUProfiles(oldVGPUObj, newVGPUObj *devicesv1beta1.VGPUDevice) error {
	if newVGPUObj.Spec.VGPUTypeName == "" {
		return fmt.Errorf("VGPUTypeName cannot be empty for vGPU device %s", newVGPUObj.Name)
	}

	if _, ok := oldVGPUObj.Status.AvailableTypes[newVGPUObj.Spec.VGPUTypeName]; !ok {
		return fmt.Errorf("VGPUTypeName %s is not a valid profile for vGPU device %s", newVGPUObj.Spec.VGPUTypeName, newVGPUObj.Name)
	}
	return nil
}
