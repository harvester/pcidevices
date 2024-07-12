package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

var (
	usbdevice3 = &devicesv1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdevice3",
		},
		Status: devicesv1beta1.USBDeviceStatus{
			NodeName:     "node1",
			ResourceName: "fake.com/device1",
			VendorID:     "8086",
			ProductID:    "1166",
			DevicePath:   "/dev/bus/002/001",
		},
	}
)

func Test_DeleteUSBDeviceInUse(t *testing.T) {
	usbdevice3.Status.Enabled = true
	assert := require.New(t)
	usbValidator := NewUSBDeviceValidator()
	err := usbValidator.Delete(nil, usbdevice3)
	assert.Error(err, "expected to get error")
	assert.Equal("usbdevice usbdevice3 is still in use", err.Error())
}

func Test_DeleteUSBDeviceNotInUse(t *testing.T) {
	usbdevice3.Status.Enabled = false
	assert := require.New(t)
	usbValidator := NewUSBDeviceValidator()
	err := usbValidator.Delete(nil, usbdevice3)
	assert.NoError(err)
}
