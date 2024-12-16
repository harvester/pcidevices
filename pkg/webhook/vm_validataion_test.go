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
		before func(usbdevice2innode1Cp *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, vmWithTwoInSameNodeDevicesCp *kubevirtv1.VirtualMachine)
	}{
		{
			name:   "matched node name",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, _ *kubevirtv1.VirtualMachine) {},
			err:    nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func(usbdevice2innode1Cp *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, vmWithTwoInSameNodeDevicesCp *kubevirtv1.VirtualMachine) {
				usbdevice2innode1Cp.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				vmWithTwoInSameNodeDevicesCp.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       pcideviceinnode1Cp.Name,
						DeviceName: pcideviceinnode1Cp.Status.ResourceName,
					},
					{
						Name:       usbdevice2innode1Cp.Name,
						DeviceName: usbdevice2innode1Cp.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, _ *kubevirtv1.VirtualMachine) {
				pcideviceinnode1Cp.Status.NodeName = "node2"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "usb device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].Name = "tempusbdevice"
			},
			err: nil,
		},
		{
			name: "mismatched usb resource name ",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice usbdevice2innode1: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "pci device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].Name = "temppcidevice"
			},
			err: nil,
		},
		{
			name: "mismatched pci resource name ",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice node1dev1noiommu: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "gpu device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1cp *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       pcideviceinnode1cp.Name + "fake",
						DeviceName: pcideviceinnode1cp.Status.ResourceName,
					},
				}
			},
			err: nil,
		},
		{
			name: "mismatched gpu resource name ",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1cp *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       pcideviceinnode1cp.Name,
						DeviceName: pcideviceinnode1cp.Status.ResourceName + "fake",
					},
				}
			},
			err: errors.New("gpu device node1dev1noiommu: resource name fake.com/device1fake not found in pcidevice cache"),
		},
	}

	for _, tc := range testcases {
		pcideviceinnode1Cp := pcideviceinnode1.DeepCopy()
		usbdevice2innode1Cp := usbdevice2innode1.DeepCopy()
		vmWithTwoInSameNodeDevicesCp := vmWithTwoInSameNodeDevices.DeepCopy()
		tc.before(
			usbdevice2innode1Cp,
			pcideviceinnode1Cp,
			vmWithTwoInSameNodeDevicesCp,
		)

		fakeClient := fake.NewSimpleClientset(usbdevice2innode1Cp, pcideviceinnode1Cp)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)

		validator := NewDeviceHostValidation(usbCache, pciCache)
		err := validator.Create(nil, vmWithTwoInSameNodeDevicesCp)

		assert.Equal(t, tc.err, err, tc.name)
	}
}

func Test_UpdateVM(t *testing.T) {

	testcases := []struct {
		name   string
		err    error
		before func(usbdevice2innode1Cp *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, vmWithTwoInSameNodeDevicesCp *kubevirtv1.VirtualMachine)
	}{
		{
			name: "matched node name",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, _ *kubevirtv1.VirtualMachine) {
			},
			err: nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func(usbdevice2innode1Cp *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, vmWithTwoInSameNodeDevicesCp *kubevirtv1.VirtualMachine) {
				usbdevice2innode1Cp.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				vmWithTwoInSameNodeDevicesCp.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       pcideviceinnode1Cp.Name,
						DeviceName: pcideviceinnode1Cp.Status.ResourceName,
					},
					{
						Name:       usbdevice2innode1Cp.Name,
						DeviceName: usbdevice2innode1Cp.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1Cp *devicesv1beta1.PCIDevice, _ *kubevirtv1.VirtualMachine) {
				pcideviceinnode1Cp.Status.NodeName = "node2"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "usb device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].Name = "tempusbdevice"
			},
			err: nil,
		},
		{
			name: "mismatched usb resource name ",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice usbdevice2innode1: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "pci device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].Name = "temppcidevice"
			},
			err: nil,
		},
		{
			name: "mismatched pci resource name ",
			before: func(_ *devicesv1beta1.USBDevice, _ *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice node1dev1noiommu: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "gpu device name is different from CR, it should be able to create",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1cp *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       pcideviceinnode1cp.Name + "fake",
						DeviceName: pcideviceinnode1cp.Status.ResourceName,
					},
				}
			},
			err: nil,
		},
		{
			name: "mismatched gpu resource name ",
			before: func(_ *devicesv1beta1.USBDevice, pcideviceinnode1cp *devicesv1beta1.PCIDevice, vm *kubevirtv1.VirtualMachine) {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       pcideviceinnode1cp.Name,
						DeviceName: pcideviceinnode1cp.Status.ResourceName + "fake",
					},
				}
			},
			err: errors.New("gpu device node1dev1noiommu: resource name fake.com/device1fake not found in pcidevice cache"),
		},
	}

	for _, tc := range testcases {
		pcideviceinnode1Cp := pcideviceinnode1.DeepCopy()
		usbdevice2innode1Cp := usbdevice2innode1.DeepCopy()
		vmWithTwoInSameNodeDevicesCp := vmWithTwoInSameNodeDevices.DeepCopy()

		tc.before(usbdevice2innode1Cp, pcideviceinnode1Cp, vmWithTwoInSameNodeDevicesCp)

		fakeClient := fake.NewSimpleClientset(usbdevice2innode1Cp, pcideviceinnode1Cp)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)

		validator := NewDeviceHostValidation(usbCache, pciCache)
		err := validator.Update(nil, nil, vmWithTwoInSameNodeDevicesCp)

		assert.Equal(t, tc.err, err, tc.name)
	}
}
