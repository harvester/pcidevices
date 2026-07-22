package webhook

import (
	"testing"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

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
	node1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}

	parentGPU = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1gpu1",
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:04:10.0",
			ClassID:           "0300",
			Description:       "fake GPU device 1",
			NodeName:          "node1",
			ResourceName:      "fake.com/gpudevice1",
			VendorID:          "8086",
			KernelDriverInUse: "vfio-pci",
			IOMMUGroup:        "15",
		},
	}
	vGPUDevice = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1vgpu1",
			Labels: map[string]string{
				devicesv1beta1.ParentSRIOVGPUDeviceLabel: "node1gpu1",
			},
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:04:10.1",
			ClassID:           "0300",
			Description:       "fake vGPU device 1",
			NodeName:          "node1",
			ResourceName:      "fake.com/vgpudevice1",
			VendorID:          "8086",
			KernelDriverInUse: "vfio-pci",
			IOMMUGroup:        "15",
		},
	}

	parentGPUClaim = &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1gpu1",
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			UserName: "admin",
			NodeName: "node1",
			Address:  "0000:04:10.0",
		},
	}

	devicesNodeObj = &devicesv1beta1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		Status: devicesv1beta1.NodeStatus{
			GPUAddresses: []string{"0000:04:10.0"},
		},
	}

	k8sClient = k8sfake.NewClientset(node1)
	nodeCache = fakeclients.NodeCache(k8sClient.CoreV1().Nodes)
)

func Test_PCIDeviceClaimWithoutIommu(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim, node1NoIommuDev, devicesNodeObj)

	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Create(nil, node1NoIommuClaim)
	assert.Error(err, "expected to find error")
}

func Test_PCIDeviceClaimWithIommu(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1NoIommuDev, devicesNodeObj)

	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Create(nil, node1dev1Claim)
	assert.NoError(err, "expected to find no error")
}

func Test_CreatePCIDeviceClaimWhenUSBInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, usbDeviceClaim1, devicesNodeObj)

	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	nodeCache := fakeclients.NodeCache(k8sClient.CoreV1().Nodes)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Create(nil, node1dev1Claim)
	assert.Error(err, "expected to get error")
}

func Test_DeletePCIDeviceClaimInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, devicesNodeObj)

	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithIommuDevice)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, vmCache, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Delete(nil, node1dev1Claim)
	assert.Error(err, "expected to get error")
}

func Test_DeletePCIDeviceClaimNotInUse(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, devicesNodeObj)

	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithoutValidDeviceName)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, vmCache, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Delete(nil, node1dev1Claim)
	assert.NoError(err, "expected no error during validation")
}

func Test_DeletePCIDeviceClaimInUseOnDeletedNode(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, devicesNodeObj)

	harvesterfakeClient := harvesterfake.NewSimpleClientset(vmWithIommuDevice)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	vmCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, vmCache, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Delete(nil, node2dev1Claim)
	assert.NoError(err, "expected no error during validation")
}

func Test_CreatePCIDeviceClaimWhenVGPUExist(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(vGPUDevice, parentGPU, devicesNodeObj)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)
	err := pciValidator.Create(nil, parentGPUClaim)
	assert.Error(err, "expected to get error")
}

func Test_CreateGPUDeviceClaimForBaremetalWorkloadNode(t *testing.T) {
	assert := require.New(t)
	deviceNodeObjCopy := devicesNodeObj.DeepCopy()
	if deviceNodeObjCopy.Labels == nil {
		deviceNodeObjCopy.Labels = make(map[string]string)
	}
	deviceNodeObjCopy.Labels[devicesv1beta1.GPUContainerWorkloadKey] = devicesv1beta1.GPUContainerWorkloadValue
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1NoIommuDev, deviceNodeObjCopy)

	usbDeviceClaimCache := fakeclients.USBDeviceClaimsCache(fakeClient.DevicesV1beta1().USBDeviceClaims)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	usbDeviceCache := fakeclients.USBDeviceCache(fakeClient.DevicesV1beta1().USBDevices)
	devicesNodeCache := fakeclients.NodeDevicesCache(fakeClient.DevicesV1beta1().Nodes)
	pciValidator := NewPCIDeviceClaimValidator(pciDeviceCache, nil, usbDeviceClaimCache, usbDeviceCache, nodeCache, devicesNodeCache)

	err := pciValidator.Create(nil, node1dev1Claim)
	assert.Error(err, "expected to get error")
}
