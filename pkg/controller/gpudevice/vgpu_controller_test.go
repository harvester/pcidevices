package gpudevice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	missingVGPU = &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000009004",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.VGPUDeviceSpec{
			NodeName:               nodeName,
			Address:                "0000:09:00.4",
			ParentGPUDeviceAddress: "0000:09:00.0",
			Enabled:                false,
		},
	}

	foundVGPU = &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000010004",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.VGPUDeviceSpec{
			NodeName:               nodeName,
			Address:                "0000:10:00.4",
			ParentGPUDeviceAddress: "0000:10:00.0",
			Enabled:                false,
		},
	}

	foundPresentVGPU = &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000010004",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.VGPUDeviceSpec{
			NodeName:               nodeName,
			Address:                "0000:10:00.4",
			ParentGPUDeviceAddress: "0000:10:00.0",
			Enabled:                true,
			VGPUTypeName:           "NVIDIA A2-4C",
		},
	}

	newVGPU = &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000011004",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.VGPUDeviceSpec{
			NodeName:               nodeName,
			Address:                "0000:11:00.4",
			ParentGPUDeviceAddress: "0000:11:00.0",
			Enabled:                false,
		},
	}
)

func Test_reconcileVGPUSetup(t *testing.T) {
	assert := require.New(t)
	client := fake.NewSimpleClientset(missingVGPU)
	var vGPUDevices []*v1beta1.VGPUDevice
	vGPUDevices = append(vGPUDevices, foundPresentVGPU, newVGPU)
	h := &Handler{
		nodeName:            nodeName,
		sriovGPUCache:       fakeclients.SriovGPUDevicesCache(client.DevicesV1beta1().SRIOVGPUDevices),
		vGPUCache:           fakeclients.VGPUDeviceCache(client.DevicesV1beta1().VGPUDevices),
		vGPUClient:          fakeclients.VGPUDeviceClient(client.DevicesV1beta1().VGPUDevices),
		pciDeviceClaimCache: fakeclients.PCIDeviceClaimsCache(client.DevicesV1beta1().PCIDeviceClaims),
	}
	err := h.reconcileVGPUSetup(vGPUDevices)
	assert.NoError(err)
	// check missing VGPU is gone
	_, err = client.DevicesV1beta1().VGPUDevices().Get(context.TODO(), missingVGPU.Name, metav1.GetOptions{})
	assert.True(apierrors.IsNotFound(err), "expected to find IsNotFound error")
	// check present VGPU is updated
	obj, err := client.DevicesV1beta1().VGPUDevices().Get(context.TODO(), foundVGPU.Name, metav1.GetOptions{})
	assert.NoError(err, "expect no error during lookup of presentVGPU")
	assert.True(obj.Spec.Enabled, "expected presentVGPU to be updated")
	assert.NotEmpty(obj.Spec.VGPUTypeName, "expected VGPUTypeName to be updated")
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}
	selector := labels.SelectorFromSet(set)
	vGPUList, err := client.DevicesV1beta1().VGPUDevices().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	assert.NoError(err, "expect no error while listing VGPU's")
	assert.Len(vGPUList.Items, 2, "expected to find only 2 VGPU's")

}

func Test_splitArr(t *testing.T) {
	arr := []string{"a", "b", "c", "d"}
	find := "c"
	for i, v := range arr {
		if v == find {
			arr = append(arr[:i], arr[i+1:]...)
		}
	}
	t.Log(arr)
}
