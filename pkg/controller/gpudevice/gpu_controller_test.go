package gpudevice

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/require"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	nodeName = "fakeNode"

	missingGPU = &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000009000",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  "0000:09:00:0",
			NodeName: nodeName,
			Enabled:  false,
		},
	}

	presentGPU = &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000010000",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  "0000:10:00:0",
			NodeName: nodeName,
			Enabled:  false,
		},
	}

	foundPresentGPU = &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000010000",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  "0000:10:00:0",
			NodeName: nodeName,
			Enabled:  true,
		},
	}

	newGPU = &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fakeNode-000011000",
			Labels: map[string]string{
				"nodename": nodeName,
			},
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  "0000:11:00:0",
			NodeName: nodeName,
			Enabled:  false,
		},
	}
)

func Test_reconcileSRIOVGPUSetup(t *testing.T) {
	assert := require.New(t)

	client := fake.NewSimpleClientset(missingGPU)
	var gpuDevices []*v1beta1.SRIOVGPUDevice
	gpuDevices = append(gpuDevices, foundPresentGPU, newGPU)
	h := &Handler{
		nodeName:            nodeName,
		sriovGPUCache:       fakeclients.SriovGPUDevicesCache(client.DevicesV1beta1().SRIOVGPUDevices),
		sriovGPUClient:      fakeclients.SriovGPUDevicesClient(client.DevicesV1beta1().SRIOVGPUDevices),
		pciDeviceClaimCache: fakeclients.PCIDeviceClaimsCache(client.DevicesV1beta1().PCIDeviceClaims),
	}

	err := h.reconcileSRIOVGPUSetup(gpuDevices)
	assert.NoError(err)
	// check missing GPU is gone
	_, err = client.DevicesV1beta1().SRIOVGPUDevices().Get(context.TODO(), missingGPU.Name, metav1.GetOptions{})
	assert.True(apierrors.IsNotFound(err), "expected to find IsNotFound error")
	// check present GPU is updated
	obj, err := client.DevicesV1beta1().SRIOVGPUDevices().Get(context.TODO(), presentGPU.Name, metav1.GetOptions{})
	assert.NoError(err, "expected no error during lookup of presentGPU")
	assert.True(obj.Spec.Enabled, "expected presentGPU to be updated")
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}
	selector := labels.SelectorFromSet(set)
	gpuList, err := client.DevicesV1beta1().SRIOVGPUDevices().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	assert.NoError(err, "expected no error while listing GPU's")
	assert.Len(gpuList.Items, 2, "expected to find only 2 GPU's")
}
