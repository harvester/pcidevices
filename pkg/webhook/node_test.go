package webhook

import (
	"testing"

	harvFake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	nodeObj = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-node",
			Labels: map[string]string{},
		},
	}

	sriovGPUObj = &devicesv1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node-sriovgpu",
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			NodeName: "test-node",
			Enabled:  true,
		},
	}

	podObj = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(devicesv1beta1.GPUResourceName): resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
)

func Test_VerifyEnableContainerWorkloads(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(sriovGPUObj)
	sriovGPUCache := fakeclients.SriovGPUDevicesCache(fakeClient.DevicesV1beta1().SRIOVGPUDevices)
	nodeValidator := NewNodeValidator(sriovGPUCache, nil)

	nodeObjCopy := nodeObj.DeepCopy()
	nodeObjCopy.Labels[devicesv1beta1.GPUContainerWorkloadKey] = devicesv1beta1.GPUContainerWorkloadValue

	err := nodeValidator.Update(nil, nodeObj, nodeObjCopy)
	assert.Error(err, "expected to find error when applying container workloads label on node with enabled sriovGPUDevices")
}

func Test_VerifyDisableContainerWorkloads(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(sriovGPUObj)
	harvFakeClient := harvFake.NewSimpleClientset(podObj)
	sriovGPUCache := fakeclients.SriovGPUDevicesCache(fakeClient.DevicesV1beta1().SRIOVGPUDevices)
	podCache := fakeclients.PodCache(harvFakeClient.CoreV1().Pods)
	nodeValidator := NewNodeValidator(sriovGPUCache, podCache)

	nodeObjCopy := nodeObj.DeepCopy()
	nodeObjCopy.Labels[devicesv1beta1.GPUContainerWorkloadKey] = devicesv1beta1.GPUContainerWorkloadValue

	err := nodeValidator.Update(nil, nodeObjCopy, nodeObj)
	assert.Error(err, "expected to find error when removing container workloads label on node with pod workloads leveraging GPU resources")
}
