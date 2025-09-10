package webhook

import (
	"fmt"
	"reflect"

	kubevirtctl "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester/pkg/webhook/types"
	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type sriovGPUValidator struct {
	types.DefaultValidator
	kubevirtCache kubevirtctl.VirtualMachineCache
	nodeCache     ctlcorev1.NodeCache
}

func NewSRIOVGPUValidator(kubevirtCache kubevirtctl.VirtualMachineCache, nodeCache ctlcorev1.NodeCache) types.Validator {
	return &sriovGPUValidator{
		kubevirtCache: kubevirtCache,
		nodeCache:     nodeCache,
	}
}

func (v *sriovGPUValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"sriovgpudevices"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.SRIOVGPUDevice{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Update,
			admissionregv1.Delete,
		},
	}
}

func (v *sriovGPUValidator) Update(_ *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldGPUObj := oldObj.(*devicesv1beta1.SRIOVGPUDevice)
	newGPUObj := newObj.(*devicesv1beta1.SRIOVGPUDevice)

	if reflect.DeepEqual(oldGPUObj.Spec, newGPUObj.Spec) {
		return nil
	}

	// vGPU was disabled, check if the device is in use with a vm
	if oldGPUObj.Spec.Enabled && !newGPUObj.Spec.Enabled {
		return v.checkGPUUsage(newGPUObj)
	}

	return nil
}

func (v *sriovGPUValidator) Delete(_ *types.Request, obj runtime.Object) error {
	gpuObj := obj.(*devicesv1beta1.SRIOVGPUDevice)

	ok, err := isNodeDeleted(v.nodeCache, gpuObj.Spec.NodeName)
	if err != nil {
		err := fmt.Errorf("error looking up node for SRIOVGPU %s from node cache: %w", gpuObj.Name, err)
		logrus.Error(err)
		return err
	}

	// node related to sriovgpu is no longer present, no need to validate further
	// allow deletion of object
	if ok {
		return nil
	}

	if gpuObj.Spec.Enabled {
		return fmt.Errorf("please disable gpuDevice %s before deletion", gpuObj.Name)
	}
	return nil
}

func (v *sriovGPUValidator) checkGPUUsage(obj *devicesv1beta1.SRIOVGPUDevice) error {
	for _, vgpu := range obj.Status.VGPUDevices {
		if err := checkVGPUUsage(v.kubevirtCache, vgpu); err != nil {
			return err
		}
	}
	return nil
}
