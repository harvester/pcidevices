package webhook

import (
	"fmt"

	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	VMLabel = "harvesterhci.io/vmName"
)

var matchingLabels = []labels.Set{
	{
		"kubevirt.io": "virt-launcher",
	},
}

func NewPodMutator(cache v1beta1.PCIDeviceClaimCache) types.Mutator {
	return &podMutator{
		claimCache: cache,
	}
}

// podMutator injects Harvester settings like http proxy envs and trusted CA certs to system pods that may access
// external services. It includes harvester apiserver and longhorn backing-image-data-source pods.
type podMutator struct {
	types.DefaultMutator
	claimCache v1beta1.PCIDeviceClaimCache
}

func newResource(ops []admissionregv1.OperationType) types.Resource {
	return types.Resource{
		Names:          []string{string(corev1.ResourcePods)},
		Scope:          admissionregv1.NamespacedScope,
		APIGroup:       corev1.SchemeGroupVersion.Group,
		APIVersion:     corev1.SchemeGroupVersion.Version,
		ObjectType:     &corev1.Pod{},
		OperationTypes: ops,
	}
}

func (m *podMutator) Resource() types.Resource {
	return newResource([]admissionregv1.OperationType{
		admissionregv1.Create,
	})
}

func (m *podMutator) Create(request *types.Request, newObj runtime.Object) (types.PatchOps, error) {
	pod := newObj.(*corev1.Pod)

	podLabels := labels.Set(pod.Labels)
	var match bool
	for _, v := range matchingLabels {
		if v.AsSelector().Matches(podLabels) {
			match = true
			break
		}
	}
	if !match {
		logrus.Infof("ignoring pod %s in ns %s as no valid labels found", pod.Name, pod.Namespace)
		return nil, nil
	}

	var patchOps types.PatchOps

	vmName, ok := pod.Labels[VMLabel]
	if !ok {
		return nil, nil
	}

	device, err := m.claimCache.GetByIndex(PCIClaimByVM, vmName)
	if err != nil {
		logrus.Errorf("error looking up deviceclaim by vmName: %v", err)
		return nil, fmt.Errorf("error looking up deviceclaim by vmName: %v", err)
	}

	if len(device) > 0 {
		capPatchOptions, err := createCapabilityPatch(pod)
		if err != nil {
			logrus.Infof("error creating capability patch for pod %s in ns %s %v", pod.Name, pod.Namespace, err)
			return nil, fmt.Errorf("error creating capability patch: %v", err)
		}

		patchOps = append(patchOps, capPatchOptions...)
	} else {
		logrus.Infof("no deviceclaim found by owner vm: %s, nothing to do", vmName)
	}

	logrus.Debugf("patch generated %v, for pod %s in ns %s", patchOps, pod.Name, pod.Namespace)

	return patchOps, nil
}

func createCapabilityPatch(pod *corev1.Pod) (types.PatchOps, error) {
	var patchOps types.PatchOps
	for idx, container := range pod.Spec.Containers {
		addPatch, err := resourcePatch(container.SecurityContext.Capabilities.Add, fmt.Sprintf("/spec/containers/%d/securityContext/capabilities/add", idx))
		if err != nil {
			return nil, err
		}
		patchOps = append(patchOps, addPatch...)
	}

	return patchOps, nil
}

func resourcePatch(add []corev1.Capability, basePath string) (types.PatchOps, error) {
	var patchOps types.PatchOps
	if len(add) == 0 {
		basePath = basePath + "/-"
	}

	value := append(add, "SYS_RESOURCE")
	valueStr, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	patchOps = append(patchOps, fmt.Sprintf(`{"op": "add", "path": "%s", "value": %s}`, basePath, valueStr))
	return patchOps, err
}
