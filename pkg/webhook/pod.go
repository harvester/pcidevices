package webhook

import (
	"fmt"

	kubevirtctl "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	VMLabel = "harvesterhci.io/vmName"
)

var matchingLabels = []labels.Set{
	{
		"kubevirt.io": "virt-launcher",
	},
}

func NewPodMutator(deviceCache v1beta1.PCIDeviceCache, kubevirtCache kubevirtctl.VirtualMachineCache) types.Mutator {
	return &podMutator{
		deviceCache:   deviceCache,
		kubevirtCache: kubevirtCache,
	}
}

// podMutator injects Harvester settings like http proxy envs and trusted CA certs to system pods that may access
// external services. It includes harvester apiserver and longhorn backing-image-data-source pods.
type podMutator struct {
	types.DefaultMutator
	deviceCache   v1beta1.PCIDeviceCache
	kubevirtCache kubevirtctl.VirtualMachineCache
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

	// indexer users vmName + Namespace to uniquely index vm's
	vm, err := m.kubevirtCache.GetByIndex(VMByName, fmt.Sprintf("%s-%s", vmName, pod.Namespace))
	if err != nil {
		logrus.Errorf("error looking up kubevirt vm %s in namespace %s: %v", vmName, pod.Namespace, err)
		return nil, fmt.Errorf("error lookup up vm: %v", err)
	}

	if len(vm) != 1 {
		return nil, fmt.Errorf("expected to find exactly 1 vm but found %d", len(vm))
	}

	var found bool

	if len(vm[0].Spec.Template.Spec.Domain.Devices.HostDevices) == 0 {
		logrus.Infof("vm %s in ns %s has no device attachments, skipping", vm[0].Name, vm[0].Namespace)
		return nil, nil
	}

	for _, v := range vm[0].Spec.Template.Spec.Domain.Devices.HostDevices {
		hostDevices, err := m.deviceCache.GetByIndex(PCIDeviceByResourceName, v.DeviceName)
		if err != nil {
			logrus.Errorf("error listing pcidevices by deviceName for vm %s in ns %s: %v", vm[0].Name, vm[0].Namespace, err)
			return nil, fmt.Errorf("error listing pcidevices by deviceName: %v", err)
		}

		if len(hostDevices) > 0 {
			found = true
			break
		}
	}

	if found {
		capPatchOptions, err := createCapabilityPatch(pod)
		if err != nil {
			logrus.Errorf("error creating capability patch for pod %s in ns %s %v", pod.Name, pod.Namespace, err)
			return nil, fmt.Errorf("error creating capability patch: %v", err)
		}

		patchOps = append(patchOps, capPatchOptions...)
	} else {
		logrus.Infof("no device found in vm: %s, for matching pcidevices", vmName)
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
