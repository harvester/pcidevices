package nodecleanup

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	currentTime = metav1.Now()
	node1       = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node9",
			DeletionTimestamp: &currentTime,
		},
	}
	fakeClient = fake.NewSimpleClientset()
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

	_, err := h.OnRemove("", node1)
	assert.NoError(err, "expected no error while reconcilling devices")
}
