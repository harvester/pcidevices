package webhook

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	usbdevice2 = &devicesv1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdevice2",
		},
		Status: devicesv1beta1.USBDeviceStatus{
			NodeName:     "node1",
			ResourceName: "fake.com/device1",
			VendorID:     "8086",
			ProductID:    "1166",
			DevicePath:   "/dev/bus/002/001",
		},
	}

	vmiWithUSBDeviceTemplate = &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-with-usb-devices2",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Devices: kubevirtv1.Devices{
					HostDevices: []kubevirtv1.HostDevice{
						{
							Name:       usbdevice2.Name,
							DeviceName: usbdevice2.Status.ResourceName,
						},
					},
				},
			},
		},
	}
)

func Test_CreateVMI(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(usbdevice2)
	usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	validator := NewDeviceHostValidation(usbCache)

	testcases := []struct {
		name   string
		labels map[string]string
		err    error
	}{
		{
			name: "matched node name",
			labels: map[string]string{
				"kubevirt.io/nodeName": "node1",
			},
			err: nil,
		},
		{
			name: "mismatched node name",
			labels: map[string]string{
				"kubevirt.io/nodeName": "non-existed-node",
			},
			err: errors.New("USB device usbdevice2 is not on the same node as VirtualMachineInstance vm-with-usb-devices2"),
		},
	}

	for _, tc := range testcases {
		vmiWithUSBDeviceTemplate.Labels = tc.labels
		err := validator.Create(nil, vmiWithUSBDeviceTemplate)
		assert.Equal(t, tc.err, err, tc.name)
	}
}

func Test_UpdateVMI(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(usbdevice2)
	usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	validator := NewDeviceHostValidation(usbCache)

	testcases := []struct {
		name   string
		labels map[string]string
		err    error
	}{
		{
			name: "matched node name",
			labels: map[string]string{
				"kubevirt.io/nodeName": "node1",
			},
			err: nil,
		},
		{
			name: "mismatched node name",
			labels: map[string]string{
				"kubevirt.io/nodeName": "non-existed-node",
			},
			err: errors.New("USB device usbdevice2 is not on the same node as VirtualMachineInstance vm-with-usb-devices2"),
		},
	}

	for _, tc := range testcases {
		vmiWithUSBDeviceTemplate.Labels = tc.labels
		err := validator.Update(nil, nil, vmiWithUSBDeviceTemplate)
		assert.Equal(t, tc.err, err, tc.name)
	}
}
