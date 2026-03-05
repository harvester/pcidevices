package gpudevice

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
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

	vguDeviceJson = `{
    "apiVersion": "devices.harvesterhci.io/v1beta1",
    "kind": "VGPUDevice",
    "metadata": {
        "creationTimestamp": "2026-02-24T09:19:08Z",
        "generation": 33,
        "labels": {
            "harvesterhci.io/parentSRIOVGPUDevice": "hp-195-000026000",
            "nodename": "hp-195"
        },
        "name": "hp-195-000026004",
        "resourceVersion": "1005259",
        "uid": "65a31c2f-d251-4e18-a201-50c939f94ce0"
    },
    "spec": {
        "address": "0000:26:00.4",
        "enabled": true,
		"vGPUTypeName": "NVIDIA L40S-12A",
        "nodeName": "hp-195",
        "parentGPUDeviceAddress": "0000:26:00.0"
    },
    "status": {
        "availableTypes": {
            "NVIDIA L40S-12A": "1163",
            "NVIDIA L40S-12Q": "1153",
            "NVIDIA L40S-16A": "1164",
            "NVIDIA L40S-16Q": "1154",
            "NVIDIA L40S-1A": "1157",
            "NVIDIA L40S-1B": "1145",
            "NVIDIA L40S-1Q": "1147",
            "NVIDIA L40S-24A": "1165",
            "NVIDIA L40S-24Q": "1155",
            "NVIDIA L40S-2A": "1158",
            "NVIDIA L40S-2B": "1146",
            "NVIDIA L40S-2Q": "1148",
            "NVIDIA L40S-3A": "1159",
            "NVIDIA L40S-3B": "2164",
            "NVIDIA L40S-3Q": "1149",
            "NVIDIA L40S-48A": "1166",
            "NVIDIA L40S-48Q": "1156",
            "NVIDIA L40S-4A": "1160",
            "NVIDIA L40S-4Q": "1150",
            "NVIDIA L40S-6A": "1161",
            "NVIDIA L40S-6Q": "1151",
            "NVIDIA L40S-8A": "1162",
            "NVIDIA L40S-8Q": "1152"
        }
    }
}`

	pcideviceJson = `{
    "apiVersion": "devices.harvesterhci.io/v1beta1",
    "kind": "PCIDevice",
    "metadata": {
        "annotations": {
            "harvesterhci.io/pcideviceDriver": "nvidia"
        },
        "creationTimestamp": "2026-02-24T05:22:25Z",
        "generation": 1,
        "labels": {
            "nodename": "hp-195"
        },
        "name": "hp-195-000026004",
        "resourceVersion": "997962",
        "uid": "febf9d89-cd82-4fd5-982a-e8ff0241614d"
    },
    "spec": {},
    "status": {
        "address": "0000:26:00.4",
        "classId": "0302",
        "description": "3D controller: NVIDIA Corporation AD102GL [L40S]",
        "deviceId": "26b9",
        "iommuGroup": "201",
        "kernelDriverInUse": "nvidia",
        "nodeName": "hp-195",
        "resourceName": "",
        "vendorId": "10de"
    }
}`

	cleanupVGPUJson = `{
    "apiVersion": "devices.harvesterhci.io/v1beta1",
    "kind": "VGPUDevice",
    "metadata": {
        "creationTimestamp": "2026-02-24T09:19:08Z",
        "generation": 4,
        "labels": {
            "harvesterhci.io/parentSRIOVGPUDevice": "hp-195-000026000",
            "nodename": "hp-195"
        },
        "name": "hp-195-000026007",
        "resourceVersion": "1049710",
        "uid": "4b185a61-7874-4c73-aa5d-7b52d3a9d350"
    },
    "spec": {
        "address": "0000:26:00.7",
        "enabled": true,
        "nodeName": "hp-195",
        "parentGPUDeviceAddress": "0000:26:00.0",
        "vGPUTypeName": "NVIDIA L40S-1A"
    },
    "status": {
        "uuid": "1157",
        "vGPUStatus": "vGPUConfigured"
    }
}`

	cleanupPCIDeviceClaim = `{
    "apiVersion": "devices.harvesterhci.io/v1beta1",
    "kind": "PCIDeviceClaim",
    "metadata": {
        "annotations": {
            "pcidevice.harvesterhci.io/override-resource-name": "nvidia.com/NVIDIA_L40S-1A",
            "pcidevices.harvesterhci.io/skip-vfio-binding": "true"
        },
        "creationTimestamp": "2026-02-25T03:37:51Z",
        "finalizers": [
            "wrangler.cattle.io/PCIDeviceClaimOnRemove"
        ],
        "generation": 1,
        "labels": {
            "harvesterhci.io/parentSRIOVGPUDevice": "hp-195-000026000"
        },
        "name": "hp-195-000026007",
        "ownerReferences": [
            {
                "apiVersion": "devices.harvesterhci.io/v1beta1",
                "kind": "PCIDevice",
                "name": "hp-195-000026007",
                "uid": "97651b54-5b3e-4a2f-b034-bc63fcd76c1f"
            }
        ],
        "resourceVersion": "1049716",
        "uid": "a9610a07-8073-4efb-815d-1242a71a9443"
    },
    "spec": {
        "address": "0000:26:00.7",
        "nodeName": "hp-195",
        "userName": "admin"
    },
    "status": {
        "kernelDriverToUnbind": "nvidia",
        "passthroughEnabled": true
    }
}`

	cleanupPCIDevice = `{
    "apiVersion": "devices.harvesterhci.io/v1beta1",
    "kind": "PCIDevice",
    "metadata": {
        "annotations": {
            "harvesterhci.io/pcideviceDriver": "nvidia",
            "pcidevice.harvesterhci.io/override-resource-name": "nvidia.com/NVIDIA_L40S-1A"
        },
        "creationTimestamp": "2026-02-24T05:22:28Z",
        "generation": 1,
        "labels": {
            "nodename": "hp-195"
        },
        "name": "hp-195-000026007",
        "resourceVersion": "1049679",
        "uid": "97651b54-5b3e-4a2f-b034-bc63fcd76c1f"
    },
    "spec": {},
    "status": {
        "address": "0000:26:00.7",
        "classId": "0302",
        "description": "3D controller: NVIDIA Corporation AD102GL [L40S]",
        "deviceId": "26b9",
        "iommuGroup": "204",
        "kernelDriverInUse": "nvidia",
        "nodeName": "hp-195",
        "resourceName": "nvidia.com/NVIDIA_L40S-1A",
        "vendorId": "10de"
    }
}`
)

func Test_reconcileVGPUSetup(t *testing.T) {
	assert := require.New(t)
	client := fake.NewSimpleClientset(missingVGPU)
	vGPUDevices := make([]*v1beta1.VGPUDevice, 0, 2)
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

// test validates if associated pcidevice objects are updated and pcideviceclaim objects are created
func Test_submitPCIDeviceClaim(t *testing.T) {
	assert := require.New(t)
	vgpuObj, err := generateObject(&v1beta1.VGPUDevice{}, vguDeviceJson)
	assert.NoError(err, "expected no error during generation of vgpu object")
	pciObj, err := generateObject(&v1beta1.PCIDevice{}, pcideviceJson)
	assert.NoError(err, "expected no error during generation of pcidevice object")
	client := fake.NewSimpleClientset(vgpuObj, pciObj)
	h := &Handler{
		nodeName:            nodeName,
		sriovGPUCache:       fakeclients.SriovGPUDevicesCache(client.DevicesV1beta1().SRIOVGPUDevices),
		vGPUCache:           fakeclients.VGPUDeviceCache(client.DevicesV1beta1().VGPUDevices),
		vGPUClient:          fakeclients.VGPUDeviceClient(client.DevicesV1beta1().VGPUDevices),
		pciDeviceClaimCache: fakeclients.PCIDeviceClaimsCache(client.DevicesV1beta1().PCIDeviceClaims),
		pciDeviceClaim:      fakeclients.PCIDeviceClaimsClient(client.DevicesV1beta1().PCIDeviceClaims),
		pciDevice:           fakeclients.PCIDevicesClient(client.DevicesV1beta1().PCIDevices),
		pciDeviceCache:      fakeclients.PCIDevicesCache(client.DevicesV1beta1().PCIDevices),
	}
	typedVGPU := vgpuObj.(*v1beta1.VGPUDevice)
	err = h.submitPCIDeviceClaim(typedVGPU)
	assert.NoError(err, "expected no error during reconcile of vgpu object")
	pciDeviceClaimObj, err := h.pciDeviceClaimCache.Get(typedVGPU.Name)
	assert.NoError(err, "expected no error during pcidevice claim lookup")
	_, ok := pciDeviceClaimObj.Annotations[v1beta1.PCIDeviceOverrideResourceName]
	assert.True(ok, "expected to find annotation for v1beta1.PCIDeviceOverrideResourceName")
	pciDeviceObj, err := h.pciDeviceCache.Get(typedVGPU.Name)
	assert.NoError(err, "expected no error during pcidevice lookup")
	_, ok = pciDeviceObj.Annotations[v1beta1.PCIDeviceOverrideResourceName]
	assert.True(ok, "expected to find annotation for v1beta1.PCIDeviceOverrideResourceName")
	assert.Equal(pciDeviceObj.Status.ResourceName, gpuhelper.GenerateDeviceName(typedVGPU.Spec.VGPUTypeName))
	_, ok = pciDeviceObj.Labels[v1beta1.ParentSRIOVGPUDeviceLabel]
	assert.True(ok, "expected to find label for v1beta1.ParentSRIOVGPUDeviceLabel")
}

func generateObject(obj runtime.Object, content string) (runtime.Object, error) {
	err := json.Unmarshal([]byte(content), obj)
	return obj, err
}

func Test_cleanupPCIDeviceClaim(t *testing.T) {
	assert := require.New(t)
	vgpuObj, err := generateObject(&v1beta1.VGPUDevice{}, cleanupVGPUJson)
	assert.NoError(err, "expected no error during generation of vgpu object")
	pciObj, err := generateObject(&v1beta1.PCIDevice{}, cleanupPCIDevice)
	assert.NoError(err, "expected no error during generation of pcidevice object")
	pciClaimObj, err := generateObject(&v1beta1.PCIDeviceClaim{}, cleanupPCIDeviceClaim)
	assert.NoError(err, "expected no error during generation of pcidevice claim object")
	client := fake.NewSimpleClientset(vgpuObj, pciObj, pciClaimObj)
	h := &Handler{
		nodeName:            nodeName,
		sriovGPUCache:       fakeclients.SriovGPUDevicesCache(client.DevicesV1beta1().SRIOVGPUDevices),
		vGPUCache:           fakeclients.VGPUDeviceCache(client.DevicesV1beta1().VGPUDevices),
		vGPUClient:          fakeclients.VGPUDeviceClient(client.DevicesV1beta1().VGPUDevices),
		pciDeviceClaimCache: fakeclients.PCIDeviceClaimsCache(client.DevicesV1beta1().PCIDeviceClaims),
		pciDeviceClaim:      fakeclients.PCIDeviceClaimsClient(client.DevicesV1beta1().PCIDeviceClaims),
		pciDevice:           fakeclients.PCIDevicesClient(client.DevicesV1beta1().PCIDevices),
		pciDeviceCache:      fakeclients.PCIDevicesCache(client.DevicesV1beta1().PCIDevices),
	}
	typedVGPU := vgpuObj.(*v1beta1.VGPUDevice)
	err = h.cleanupRelatedPCIDeviceObjects(typedVGPU)
	assert.NoError(err, "expected no error during reconcile of vgpu object")
	_, err = h.pciDeviceClaimCache.Get(typedVGPU.Name)
	ok := apierrors.IsNotFound(err)
	assert.True(ok, "expected to not find pcidevice claim")
	pciDeviceObj, err := h.pciDeviceCache.Get(typedVGPU.Name)
	assert.NoError(err, "expected no error during pcidevice lookup")
	_, ok = pciDeviceObj.Annotations[v1beta1.PCIDeviceOverrideResourceName]
	assert.False(ok, "expected to not find annotation for v1beta1.PCIDeviceOverrideResourceName")
	_, ok = pciDeviceObj.Labels[v1beta1.ParentSRIOVGPUDeviceLabel]
	assert.False(ok, "expected to not find label for v1beta1.ParentSRIOVGPUDeviceLabel")
}
