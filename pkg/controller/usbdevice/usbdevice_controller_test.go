package usbdevice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/deviceplugins"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var mockWalkUSBDevices = func() (map[int][]*deviceplugins.USBDevice, error) {
	return map[int][]*deviceplugins.USBDevice{
		0: {
			{
				Name:         "test",
				Manufacturer: "test",
				Vendor:       2385,
				Product:      5734,
				DevicePath:   "/dev/bus/usb/001/002",
				PCIAddress:   "0000:02:01.0",
			},
		},
	}, nil
}

var mockCommonLabel = &commonLabel{
	nodeName: "test-node",
}

func Test_ReconcileUSBDevices(t *testing.T) {
	// detect one usb device, create a USBDevice CR
	walkUSBDevices = mockWalkUSBDevices
	cl = mockCommonLabel
	client := fake.NewSimpleClientset()

	usbHandler := NewHandler(
		fakeclients.USBDevicesClient(client.DevicesV1beta1().USBDevices),
		fakeclients.USBDeviceClaimsClient(client.DevicesV1beta1().USBDeviceClaims),
	)

	err := usbHandler.ReconcileUSBDevices()
	assert.NoError(t, err)

	list, err := client.DevicesV1beta1().USBDevices().List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(list.Items))
	assert.Equal(t, "test-node-0951-1666-001002", list.Items[0].Name)
	assert.Equal(t, "0951", list.Items[0].Status.VendorID)
	assert.Equal(t, "1666", list.Items[0].Status.ProductID)
	assert.Equal(t, "kubevirt.io/test-node-0951-1666-001002", list.Items[0].Status.ResourceName)
	assert.Equal(t, "/dev/bus/usb/001/002", list.Items[0].Status.DevicePath)
	assert.Equal(t, cl.nodeName, list.Items[0].Status.NodeName)
	assert.Equal(t, "DataTraveler 100 G3/G4/SE9 G2/50 Kyson (Kingston Technology)", list.Items[0].Status.Description)

	// detect no usb device after few minutes, delete existing USBDevice CR
	walkUSBDevices = func() (map[int][]*deviceplugins.USBDevice, error) { return map[int][]*deviceplugins.USBDevice{}, nil }

	err = usbHandler.ReconcileUSBDevices()
	assert.NoError(t, err)

	list, err = client.DevicesV1beta1().USBDevices().List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(list.Items))
}
