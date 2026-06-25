package webhook

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/harvester/harvester/pkg/webhook/types"
	ctlcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type nodeValidator struct {
	types.DefaultValidator
	sriovGPUDeviceCache ctl.SRIOVGPUDeviceCache
	podCache            ctlcore.PodCache
}

func NewNodeValidator(sriovGPUDeviceCache ctl.SRIOVGPUDeviceCache, podCache ctlcore.PodCache) types.Validator {
	return &nodeValidator{
		sriovGPUDeviceCache: sriovGPUDeviceCache,
		podCache:            podCache,
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

	var oldNodeHasContainerWorkloadGPULabel, newNodeHasContainerWorkloadLabel bool
	if oldNodeObj.Labels[v1beta1.GPUContainerWorkloadKey] == v1beta1.GPUContainerWorkloadValue {
		oldNodeHasContainerWorkloadGPULabel = true
	}

	if newNodeObj.Labels[v1beta1.GPUContainerWorkloadKey] == v1beta1.GPUContainerWorkloadValue {
		newNodeHasContainerWorkloadLabel = true
	}

	// if oldNode does not have continer workload label and is being added
	// then verify there are no enabled SRIOV GPU devices on the node, if so return error
	if !oldNodeHasContainerWorkloadGPULabel && newNodeHasContainerWorkloadLabel {
		return v.CheckEnabledSRIOVGPUDevices(newNodeObj.Name)
	}

	// if oldNode has continer workload label and is being removed
	// then verify there are no pods scheduled on the node which may be using the GPU, if so return error
	if oldNodeHasContainerWorkloadGPULabel && !newNodeHasContainerWorkloadLabel {
		pods, err := v.podCache.GetByIndex(v1beta1.GPUPodsByNodeName, newNodeObj.Name)
		if err != nil {
			return err
		}

		if len(pods) > 0 {
			podNames := make([]string, 0, len(pods))
			for _, pod := range pods {
				podNames = append(podNames, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			}
			return fmt.Errorf("node %s has pods %s scheduled which may be using the GPU", newNodeObj.Name, strings.Join(podNames, ", "))
		}
	}
	return nil
}

func (v *nodeValidator) CheckEnabledSRIOVGPUDevices(nodeName string) error {
	sriovGPUDevices, err := v.sriovGPUDeviceCache.GetByIndex(v1beta1.EnabledSRIOVGPUDevicesByNodeNameIndex, nodeName)
	if err != nil {
		return err
	}

	if len(sriovGPUDevices) > 0 {
		sriovGPUDeviceNames := make([]string, 0, len(sriovGPUDevices))
		for _, device := range sriovGPUDevices {
			sriovGPUDeviceNames = append(sriovGPUDeviceNames, device.Name)
		}
		return fmt.Errorf("node %s has enabled SRIOV GPU devices %s", nodeName, strings.Join(sriovGPUDeviceNames, ", "))
	}
	return nil
}
