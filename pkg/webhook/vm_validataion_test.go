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
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
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

	vgpudeviceinnode1 = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu-upgrade-test-000008005",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Address:                "0000:08:00.5",
			Enabled:                true,
			NodeName:               "vgpu-upgrade-test",
			ParentGPUDeviceAddress: "0000:08:00.0",
			VGPUTypeName:           "NVIDIA A2-2Q",
		},
		Status: devicesv1beta1.VGPUDeviceStatus{
			ConfiguredVGPUTypeName: "NVIDIA A2-2Q",
			UUID:                   "f2285cf1-0aaa-4d05-af20-78cec22f02c7",
			VGPUStatus:             "vGPUConfigured",
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

	type input struct {
		usbDevice  *devicesv1beta1.USBDevice
		pciDevice  *devicesv1beta1.PCIDevice
		vgpuDevice *devicesv1beta1.VGPUDevice
		vm         *kubevirtv1.VirtualMachine
	}

	testcases := []struct {
		name   string
		err    error
		before func(in input)
	}{
		{
			name: "matched node name",
			before: func(_ input) {
			},
			err: nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func(in input) {
				in.usbDevice.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       in.pciDevice.Name,
						DeviceName: in.pciDevice.Status.ResourceName,
					},
					{
						Name:       in.usbDevice.Name,
						DeviceName: in.usbDevice.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func(in input) {
				in.pciDevice.Status.NodeName = "node2"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "usb device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].Name = "tempusbdevice"
			},
			err: nil,
		},
		{
			name: "mismatched usb resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice usbdevice2innode1: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "pci device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].Name = "temppcidevice"
			},
			err: nil,
		},
		{
			name: "mismatched pci resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice node1dev1noiommu: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "gpu device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				in.vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       in.vgpuDevice.Name + "fake",
						DeviceName: gpuhelper.GenerateDeviceName(in.vgpuDevice.Status.ConfiguredVGPUTypeName),
					},
				}
			},
			err: nil,
		},
		{
			name: "mismatched gpu resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				in.vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       in.vgpuDevice.Name,
						DeviceName: gpuhelper.GenerateDeviceName(in.vgpuDevice.Status.ConfiguredVGPUTypeName) + "fake",
					},
				}
			},
			err: errors.New("gpu device vgpu-upgrade-test-000008005: resource name nvidia.com/NVIDIA_A2-2Qfake not found in pcidevice cache"),
		},
	}

	for _, tc := range testcases {
		in := input{
			pciDevice:  pcideviceinnode1.DeepCopy(),
			usbDevice:  usbdevice2innode1.DeepCopy(),
			vgpuDevice: vgpudeviceinnode1.DeepCopy(),
			vm:         vmWithTwoInSameNodeDevices.DeepCopy(),
		}
		tc.before(in)

		fakeClient := fake.NewSimpleClientset(in.vgpuDevice, in.pciDevice, in.usbDevice)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
		vGPUCache := fakeclients.VGPUDeviceCache(fakeClient.DevicesV1beta1().VGPUDevices)

		validator := NewDeviceHostValidation(usbCache, pciCache, vGPUCache)
		err := validator.Create(nil, in.vm)

		assert.Equal(t, tc.err, err, tc.name)
	}
}

func Test_UpdateVM(t *testing.T) {

	type input struct {
		usbDevice  *devicesv1beta1.USBDevice
		pciDevice  *devicesv1beta1.PCIDevice
		vgpuDevice *devicesv1beta1.VGPUDevice
		vm         *kubevirtv1.VirtualMachine
	}

	testcases := []struct {
		name   string
		err    error
		before func(in input)
	}{
		{
			name: "matched node name",
			before: func(_ input) {
			},
			err: nil,
		},
		{
			name: "mismatched node name - mismatched usb device",
			before: func(in input) {
				in.usbDevice.Status.NodeName = "node2"
				// change order to trigger usb device is mismatched
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{
					{
						Name:       in.pciDevice.Name,
						DeviceName: in.pciDevice.Status.ResourceName,
					},
					{
						Name:       in.usbDevice.Name,
						DeviceName: in.usbDevice.Status.ResourceName,
					},
				}
			},
			err: errors.New("device usbdevice/usbdevice2innode1 is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "mismatched node name - mismatched pci device",
			before: func(in input) {
				in.pciDevice.Status.NodeName = "node2"
			},
			err: errors.New("device pcidevice/node1dev1noiommu is not on the same node in VirtualMachine.Spec.Template.Spec.Domain.Devices.HostDevices vm-with-usb-devices2"),
		},
		{
			name: "usb device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].Name = "tempusbdevice"
			},
			err: nil,
		},
		{
			name: "mismatched usb resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[0].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice usbdevice2innode1: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "pci device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].Name = "temppcidevice"
			},
			err: nil,
		},
		{
			name: "mismatched pci resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].DeviceName = "fake.com/device2"
			},
			err: errors.New("hostdevice node1dev1noiommu: resource name fake.com/device2 not found in pcidevice and usbdevice cache"),
		},
		{
			name: "gpu device name is different from CR, it should be able to create",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				in.vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       in.vgpuDevice.Name + "fake",
						DeviceName: gpuhelper.GenerateDeviceName(in.vgpuDevice.Status.ConfiguredVGPUTypeName),
					},
				}
			},
			err: nil,
		},
		{
			name: "mismatched gpu resource name ",
			before: func(in input) {
				in.vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{}
				in.vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
					{
						Name:       in.vgpuDevice.Name,
						DeviceName: gpuhelper.GenerateDeviceName(in.vgpuDevice.Status.ConfiguredVGPUTypeName) + "fake",
					},
				}
			},
			err: errors.New("gpu device vgpu-upgrade-test-000008005: resource name nvidia.com/NVIDIA_A2-2Qfake not found in pcidevice cache"),
		},
	}

	for _, tc := range testcases {
		in := input{
			pciDevice:  pcideviceinnode1.DeepCopy(),
			usbDevice:  usbdevice2innode1.DeepCopy(),
			vgpuDevice: vgpudeviceinnode1.DeepCopy(),
			vm:         vmWithTwoInSameNodeDevices.DeepCopy(),
		}
		tc.before(in)

		fakeClient := fake.NewSimpleClientset(in.vgpuDevice, in.pciDevice, in.usbDevice)
		usbCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
		pciCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
		vGPUCache := fakeclients.VGPUDeviceCache(fakeClient.DevicesV1beta1().VGPUDevices)

		validator := NewDeviceHostValidation(usbCache, pciCache, vGPUCache)
		err := validator.Update(nil, nil, in.vm)

		assert.Equal(t, tc.err, err, tc.name)
	}
}
