package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	node1dev1 = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev1",
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
			IOMMUGroup:        "89",
		},
	}

	node1dev1Claim = &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev1",
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			UserName: "admin",
			NodeName: "node1",
			Address:  "0000:04:10.0",
		},
	}

	node1dev2 = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev2",
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:04:10.1",
			ClassID:           "0200",
			Description:       "fake device 2",
			NodeName:          "node1",
			ResourceName:      "fake.com/device2",
			VendorID:          "8086",
			KernelDriverInUse: "ixgbevf",
			IOMMUGroup:        "89",
		},
	}

	node1dev2Claim = &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev2",
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			UserName: "admin",
			NodeName: "node1",
			Address:  "0000:04:10.1",
		},
	}

	node1dev3 = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1dev3",
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:05:10.1",
			ClassID:           "0300",
			Description:       "fake device 3",
			NodeName:          "node1",
			ResourceName:      "fake.com/device3",
			VendorID:          "8086",
			KernelDriverInUse: "ixgbevf",
			IOMMUGroup:        "99",
		},
	}

	node2dev1 = &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2dev1",
		},
		Spec: devicesv1beta1.PCIDeviceSpec{},
		Status: devicesv1beta1.PCIDeviceStatus{
			Address:           "0000:04:10.0",
			ClassID:           "0300",
			Description:       "fake device 1",
			NodeName:          "node2",
			ResourceName:      "fake.com/device1",
			VendorID:          "8086",
			KernelDriverInUse: "ixgbevf",
			IOMMUGroup:        "89",
		},
	}

	node2dev1Claim = &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2dev1",
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			UserName: "admin",
			NodeName: "node2",
			Address:  "0000:04:10.0",
		},
	}

	vmWithIommuDevice = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-with-iommu-devices",
			Namespace: "default",
			Annotations: map[string]string{
				devicesv1beta1.DeviceAllocationKey: `{"hostdevices":{"fake.com/device1":["node1dev1"]}}`,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       node1dev1.Name,
									DeviceName: node1dev1.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}

	vmWithAllIommuDevice = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-with-iommu-devices",
			Namespace: "default",
			Annotations: map[string]string{
				devicesv1beta1.DeviceAllocationKey: `{"hostdevices":{"fake.com/device1":["node1dev1"],"fake.com/device2":["node1dev2"]}}`,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       node1dev1.Name,
									DeviceName: node1dev1.Status.ResourceName,
								},
								{
									Name:       node1dev2.Name,
									DeviceName: node1dev2.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}

	vmWithoutIommuDevice = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-without-iommu-devices",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       node1dev3.Name,
									DeviceName: node1dev3.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}

	vmWithoutDevice = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-without-devices",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       "RandomName",
									DeviceName: node1dev3.Status.ResourceName,
								},
							},
						},
					},
				},
			},
		},
	}

	vmWithoutValidDeviceName = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-without-devices",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{},
						},
					},
				},
			},
		},
	}
)

func Test_VMWithNoDevices(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	pciClaimClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	pciClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)

	vmPCIMutator := &vmPCIMutator{
		deviceCache:    pciDeviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}

	patchOps, err := vmPCIMutator.generatePatch(vmWithoutDevice)
	assert.NoError(err, "expect no error while creation of patch")
	assert.Len(patchOps, 0, "expected no patch operation to be generated")
}

func Test_VMWithoutIommuDevices(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	pciClaimClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	pciClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)

	vmPCIMutator := &vmPCIMutator{
		deviceCache:    pciDeviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}

	patchOps, err := vmPCIMutator.generatePatch(vmWithoutIommuDevice)
	assert.NoError(err, "expect no error while creation of patch")
	assert.Len(patchOps, 0, "expected no patch operation to be generated")
}

func Test_VMWithIommuDevices(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	pciClaimClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	pciClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)

	vmPCIMutator := &vmPCIMutator{
		deviceCache:    pciDeviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}

	patchOps, err := vmPCIMutator.generatePatch(vmWithIommuDevice)
	assert.NoError(err, "expect no error while creation of patch")
	assert.Len(patchOps, 1, "expected patch operation to be generated")
	newPCIDeviceClaimObj, err := vmPCIMutator.pciClaimCache.Get(node1dev2.Name)
	assert.NoError(err, "expect no error while looking up claim for node1dev2")
	assert.Equal(node1dev1Claim.Spec.UserName, newPCIDeviceClaimObj.Spec.UserName, "expected username to be copied")
}

func Test_VMWithAllIommuDevices(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim, node1dev2Claim)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	pciClaimClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	pciClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)

	vmPCIMutator := &vmPCIMutator{
		deviceCache:    pciDeviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}

	patchOps, err := vmPCIMutator.generatePatch(vmWithAllIommuDevice)
	assert.NoError(err, "expect no error while creation of patch")
	assert.Len(patchOps, 0, "expected no patch operation to be generated")
}

func Test_VMWithoutValidDeviceName(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1, node1dev2, node1dev3, node2dev1, node1dev1Claim)
	pciDeviceCache := fakeclients.PCIDevicesCache(fakeClient.DevicesV1beta1().PCIDevices)
	pciClaimClient := fakeclients.PCIDeviceClaimsClient(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	pciClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)

	vmPCIMutator := &vmPCIMutator{
		deviceCache:    pciDeviceCache,
		pciClaimCache:  pciClaimCache,
		pciClaimClient: pciClaimClient,
	}

	patchOps, err := vmPCIMutator.generatePatch(vmWithoutValidDeviceName)
	assert.NoError(err, "expect no error while creation of patch")
	assert.Len(patchOps, 0, "expected no patch operation to be generated")
}
