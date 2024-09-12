package webhook

import (
	"testing"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	usbdevice1 = &devicesv1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdevice1",
		},
		Status: devicesv1beta1.USBDeviceStatus{
			NodeName:     "node1",
			ResourceName: "fake.com/device1",
			VendorID:     "8086",
			ProductID:    "1166",
			DevicePath:   "/dev/bus/002/001",
		},
	}

	usbdeviceclaim1 = &devicesv1beta1.USBDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdevice1",
		},
	}

	vmWithValidUSBDeviceName = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-with-usb-devices",
			Namespace: "default",
			Annotations: map[string]string{
				devicesv1beta1.DeviceAllocationKey: `{"hostdevices":{"fake.com/device1":["usbdevice1"]}}`,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       usbdevice1.Name,
									DeviceName: usbdevice1.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}
)

func Test_UploadUSBDeviceClaimNotInUse(t *testing.T) {
	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset()
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	usbValidator := NewUSBDeviceClaimValidator(vmCache)
	old := usbdeviceclaim1.DeepCopy()
	old.Spec.UserName = "admin"
	newOne := usbdeviceclaim1.DeepCopy()
	newOne.Spec.UserName = "admin2"

	err := usbValidator.Update(nil, old, newOne)

	assert.Error(err, "expected error when updating the userName")
}

func Test_DeleteUSBDeviceClaimInUse(t *testing.T) {
	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithValidUSBDeviceName)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	usbValidator := NewUSBDeviceClaimValidator(vmCache)
	err := usbValidator.Delete(nil, usbdeviceclaim1)
	assert.Error(err, "expected to get error")
	assert.Equal("usbdeviceclaim usbdevice1 is still in use by vm vm-with-usb-devices/default", err.Error())
}

func Test_DeleteUSBDeviceClaimNotInUse(t *testing.T) {
	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithoutValidDeviceName)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	usbValidator := NewUSBDeviceClaimValidator(vmCache)
	err := usbValidator.Delete(nil, usbdeviceclaim1)
	assert.NoError(err, "expected no error during validation")
}
