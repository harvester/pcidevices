package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
			Labels: map[string]string{
				devicesv1beta1.NodeKeyName: "test-node",
			},
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			NodeName: "test-node",
			Enabled:  true,
		},
	}
)

func Test_VerifyEnableContainerWorkloads(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(sriovGPUObj)
	sriovGPUCache := fakeclients.SriovGPUDevicesCache(fakeClient.DevicesV1beta1().SRIOVGPUDevices)
	nodeValidator := NewNodeValidator(sriovGPUCache)

	nodeObjCopy := nodeObj.DeepCopy()
	nodeObjCopy.Labels[devicesv1beta1.GPUContainerWorkloadKey] = devicesv1beta1.GPUContainerWorkloadValue

	err := nodeValidator.Update(nil, nodeObj, nodeObjCopy)
	assert.Error(err, "expected to find error when applying container workloads label on node with enabled sriovGPUDevices")
}

func Test_VerifyDisableContainerWorkloads(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(sriovGPUObj)
	sriovGPUCache := fakeclients.SriovGPUDevicesCache(fakeClient.DevicesV1beta1().SRIOVGPUDevices)
	nodeValidator := NewNodeValidator(sriovGPUCache)

	nodeObjCopy := nodeObj.DeepCopy()
	nodeObjCopy.Labels[devicesv1beta1.GPUContainerWorkloadKey] = devicesv1beta1.GPUContainerWorkloadValue

	err := nodeValidator.Update(nil, nodeObjCopy, nodeObj)
	assert.NoError(err, "expected no error when removing container workloads label on node with enabled sriovGPUDevices")
}
