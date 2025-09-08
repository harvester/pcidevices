package nodecleanup

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	currentTime = metav1.Now()
	node1       = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node1",
			DeletionTimestamp: &currentTime,
		},
	}

	node2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
	}

	vgpuDevice1 = &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-vgpu1",
			Labels: map[string]string{
				"nodename": node1.Name,
			},
		},
	}
	usbDevice1 = &v1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-usbdevice1",
			Labels: map[string]string{
				"nodename": node1.Name,
			},
		},
	}
	usbDeviceClaim1 = &v1beta1.USBDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-usbdevice1",
		},
		Status: v1beta1.USBDeviceClaimStatus{
			NodeName: node1.Name,
		},
	}

	pcidevice1 = &v1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2-pcidevice1",
			Labels: map[string]string{
				"nodename": node2.Name,
			},
		},
	}

	pcideviceclaim1 = &v1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2-pcidevice1",
		},
		Spec: v1beta1.PCIDeviceClaimSpec{
			NodeName: node2.Name,
		},
	}

	sriovNetworkDevice1 = &v1beta1.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2-sriovnetworkdevice1",
			Labels: map[string]string{
				"nodename": node2.Name,
			},
		},
	}

	fakeClient = fake.NewSimpleClientset(vgpuDevice1, usbDevice1, usbDeviceClaim1, pcidevice1, pcideviceclaim1, sriovNetworkDevice1)
)

// check deletion is not blocked if there are no device resources for specific node
func Test_ValidateRemove(t *testing.T) {
	assert := require.New(t)
	pdClient := fakeclients.PCIDevicesClient(fakeClient.DevicesV1beta1().PCIDevices)
	pdcClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovNetworkDevicesClient := fakeclients.SriovDevicesClient(fakeClient.DevicesV1beta1().SRIOVNetworkDevices)
	sriovGPUDevicesClient := fakeclients.SriovGPUDevicesClient(fakeClient.DevicesV1beta1().SRIOVGPUDevices)
	vgpuDevicesClient := fakeclients.VGPUDeviceClient(fakeClient.DevicesV1beta1().VGPUDevices)
	usbDeviceClaimClient := fakeclients.USBDeviceClaimsClient(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceClient := fakeclients.USBDevicesClient(fakeClient.DevicesV1beta1().USBDevices)
	nodeDevicesClient := fakeclients.NodeDevicesClient(fakeClient.DevicesV1beta1().Nodes)

	h := &Handler{
		pdcClient:                 pdcClient,
		pdClient:                  pdClient,
		sriovNetworkDevicesClient: sriovNetworkDevicesClient,
		sriovGPUDevicesClient:     sriovGPUDevicesClient,
		vgpuDevicesClient:         vgpuDevicesClient,
		usbDeviceClaimClient:      usbDeviceClaimClient,
		usbDevicesClient:          usbDeviceClient,
		nodeDevicesClient:         nodeDevicesClient,
	}

	// emulate deletion of node1
	_, err := h.OnRemove("", node1)
	assert.NoError(err, "expected no error while reconcilling node1")
	usbDeviceList, err := usbDeviceClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing usbdevices")
	assert.Len(usbDeviceList.Items, 0, "expected to find no usbdevices")
	vgpuDeviceList, err := vgpuDevicesClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing vgpudevices")
	assert.Len(vgpuDeviceList.Items, 0, "expected to find no vgpudevices")
	usbDeviceClaimList, err := usbDeviceClaimClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing usbdeviceclaims")
	assert.Len(usbDeviceClaimList.Items, 0, "expected to find no usbdeviceclaims")

	// emulate deletion of node2
	// no objects on node2 should be cleaned up
	_, err = h.OnRemove("", node2)
	assert.NoError(err, "expected no error while reconcilling node2")
	pciDevicesList, err := pdClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing pcidevices")
	assert.Len(pciDevicesList.Items, 1, "expected to find 1 pcidevice for node2")
	pciDeviceClaimList, err := pdcClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing pcidevicelcimas")
	assert.Len(pciDeviceClaimList.Items, 1, "expected to find 1 pcideviceclaim for node2")
	sriovNetworkDevicesList, err := sriovNetworkDevicesClient.List(metav1.ListOptions{})
	assert.NoError(err, "expected no error while listing sriovnetworkdevices")
	assert.Len(sriovNetworkDevicesList.Items, 1, "expected to find 1 sriovnetworkdevices for node2")
}
