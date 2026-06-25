package webhook

import (
	"fmt"
	"reflect"

	"github.com/harvester/harvester/pkg/webhook/types"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type nodeValidator struct {
	types.DefaultValidator
	sriovGPUDeviceCache ctl.SRIOVGPUDeviceCache
}

func NewNodeValidator(sriovGPUDeviceCache ctl.SRIOVGPUDeviceCache) types.Validator {
	return &nodeValidator{
		sriovGPUDeviceCache: sriovGPUDeviceCache,
	}
}

func (v *nodeValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"nodes"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   corev1.SchemeGroupVersion.Group,
		APIVersion: corev1.SchemeGroupVersion.Version,
		ObjectType: &corev1.Node{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Update,
		},
	}
}

func (v *nodeValidator) Update(_ *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldNodeObj := oldObj.(*corev1.Node)
	newNodeObj := newObj.(*corev1.Node)

	if reflect.DeepEqual(oldNodeObj.Labels, newNodeObj.Labels) {
		return nil
	}

	var oldDisableGPU, newDisableGPU bool
	if oldNodeObj.Labels[v1beta1.GPUContainerWorkloadKey] == v1beta1.GPUContainerWorkloadValue {
		oldDisableGPU = true
	}

	if newNodeObj.Labels[v1beta1.GPUContainerWorkloadKey] == v1beta1.GPUContainerWorkloadValue {
		newDisableGPU = true
	}

	// if label has been added on node, then we need to verify there are no enabled sriovGPUDevices on this node
	if !oldDisableGPU && newDisableGPU {
		return v.CheckEnabledSRIOVGPUDevices(newNodeObj.Name)
	}
	return nil
}

func (v *nodeValidator) CheckEnabledSRIOVGPUDevices(nodeName string) error {
	labelSet := map[string]string{
		v1beta1.NodeKeyName: nodeName,
	}
	sriovGPUSelector := labels.SelectorFromSet(labelSet)

	sriovGPUDevices, err := v.sriovGPUDeviceCache.List(sriovGPUSelector)
	if err != nil {
		return err
	}
	for _, device := range sriovGPUDevices {
		if device.Spec.Enabled {
			return fmt.Errorf("node %s has enabled SRIOV GPU devices", nodeName)
		}
	}
	return nil
}
