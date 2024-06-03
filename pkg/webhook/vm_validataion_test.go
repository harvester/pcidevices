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
	usbdevice2innode1 = &devicesv1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdevice2innode1",
		},
		Status: devicesv1beta1.USBDeviceStatus{
			NodeName:     "node1",
			ResourceName: "fake.com/device1",
			VendorID:     "8086",
			ProductID:    "1166",
			DevicePath:   "/dev/bus/002/001",
		},
	}

	pcideviceinnode1 = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev1noiommu",
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:04:10.0",
			ClassID:           "0200",
			Description:       "fake device 1",
			NodeName:          "node1",
			ResourceName:      "fake.com/device1",
			VendorID:          "8086",
			KernelDriverInUse: "ixgbevf",
			IOMMUGroup:        "",
		},
	}

	vmWithTwoInSameNodeDevices = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-with-usb-devices2",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       usbdevice2innode1.Name,
									DeviceName: usbdevice2innode1.Status.ResourceName,
								},
								{
									Name:       pcideviceinnode1.Name,
									DeviceName: pcideviceinnode1.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}
)

func Test_CreateVM(t *testing.T) {
	testcases := []struct {
		name   string
		err    error
		before func() // do some changes before testing
		after  func() // recover changes after testing
	}{
		{
			name:   "matched node name",
			before: func() {},
			after:  func() {},
			err:    nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func() {
				usbdevice2innode1.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				vmWithTwoInSameNodeDevices.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       pcideviceinnode1.Name,
						DeviceName: pcideviceinnode1.Status.ResourceName,
					},
					{
						Name:       usbdevice2innode1.Name,
						DeviceName: usbdevice2innode1.Status.ResourceName,
					},
				}
			},
			after: func() {
				usbdevice2innode1.Status.NodeName = "node1"
				vmWithTwoInSameNodeDevices.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       usbdevice2innode1.Name,
						DeviceName: usbdevice2innode1.Status.ResourceName,
					},
					{
						Name:       pcideviceinnode1.Name,
						DeviceName: pcideviceinnode1.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func() {
				pcideviceinnode1.Status.NodeName = "node2"
			},
			after: func() {
				pcideviceinnode1.Status.NodeName = "node1"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
	}

	for _, tc := range testcases {
		tc.before()

		fakeClient := fake.NewSimpleClientset(usbdevice2innode1, pcideviceinnode1)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
		validator := NewDeviceHostValidation(usbCache, pciCache)
		err := validator.Create(nil, vmWithTwoInSameNodeDevices)

		assert.Equal(t, tc.err, err, tc.name)

		tc.after()
	}
}

func Test_UpdateVM(t *testing.T) {
	testcases := []struct {
		name   string
		err    error
		before func() // do some changes before testing
		after  func() // recover changes after testing
	}{
		{
			name:   "matched node name",
			before: func() {},
			after:  func() {},
			err:    nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func() {
				usbdevice2innode1.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				vmWithTwoInSameNodeDevices.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       pcideviceinnode1.Name,
						DeviceName: pcideviceinnode1.Status.ResourceName,
					},
					{
						Name:       usbdevice2innode1.Name,
						DeviceName: usbdevice2innode1.Status.ResourceName,
					},
				}
			},
			after: func() {
				usbdevice2innode1.Status.NodeName = "node1"
				vmWithTwoInSameNodeDevices.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       usbdevice2innode1.Name,
						DeviceName: usbdevice2innode1.Status.ResourceName,
					},
					{
						Name:       pcideviceinnode1.Name,
						DeviceName: pcideviceinnode1.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func() {
				pcideviceinnode1.Status.NodeName = "node2"
			},
			after: func() {
				pcideviceinnode1.Status.NodeName = "node1"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
	}

	for _, tc := range testcases {
		tc.before()

		fakeClient := fake.NewSimpleClientset(usbdevice2innode1, pcideviceinnode1)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
		validator := NewDeviceHostValidation(usbCache, pciCache)
		err := validator.Update(nil, nil, vmWithTwoInSameNodeDevices)

		assert.Equal(t, tc.err, err, tc.name)

		tc.after()
	}
}
