package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	node1NoIommuDev = &devicesv1beta1.PCIDevice{
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

	node1NoIommuClaim = &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev1noiommu",
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			UserName: "admin",
			NodeName: "node1",
			Address:  "0000:04:10.0",
		},
	}

	usbDeviceClaim1 = &devicesv1beta1.USBDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "usbdeviceclaim1",
		},
		Status: devicesv1beta1.USBDeviceClaimStatus{
			NodeName:   "node1",
			PCIAddress: "0000:04:10.0",
		},
	}
)

func Test_PCIDeviceClaimWithoutIommu(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim, node1NoIommuDev)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)

	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache)

	err := pciValidator.Create(nil, node1NoIommuClaim)
	assert.Error(err, "expected to find error")
}

func Test_PCIDeviceClaimWithIommu(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1NoIommuDev)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)

	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache)

	err := pciValidator.Create(nil, node1dev1Claim)
	assert.NoError(err, "expected to find no error")
}

func Test_CreatePCIDeviceClaimWhenUSBInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, usbDeviceClaim1)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)

	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache)

	err := pciValidator.Create(nil, node1dev1Claim)
	assert.Error(err, "expected to get error")
}

func Test_DeletePCIDeviceClaimInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithIommuDevice)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)

	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, vmCache, usbDeviceClaimCache)

	err := pciValidator.Delete(nil, node1dev1Claim)
	assert.Error(err, "expected to get error")
}

func Test_DeletePCIDeviceClaimNotInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithoutValidDeviceName)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)

	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, vmCache, usbDeviceClaimCache)

	err := pciValidator.Delete(nil, node1dev1Claim)
	assert.NoError(err, "expected no error during validation")
}
