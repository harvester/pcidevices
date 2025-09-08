package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	devices "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	sriovDeviceEnabled = &devices.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-eno1",
		},
		Spec: devices.SRIOVNetworkDeviceSpec{
			Address:  "0000:04:00.0",
			NodeName: "node1",
			NumVFs:   1,
		},
		Status: devices.SRIOVNetworkDeviceStatus{
			VFPCIDevices: []string{node1dev1.Name, node1dev2.Name},
			VFAddresses:  []string{node1dev1.Status.Address, node1dev2.Status.Address},
		},
	}

	sriovDeviceEnabledOnDeletedNode = &devices.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2-eno1",
		},
		Spec: devices.SRIOVNetworkDeviceSpec{
			Address:  "0000:04:00.0",
			NodeName: "node2",
			NumVFs:   1,
		},
		Status: devices.SRIOVNetworkDeviceStatus{
			VFPCIDevices: []string{node2dev1.Name},
			VFAddresses:  []string{node2dev1.Status.Address},
		},
	}
)

func Test_DisableSRIOVDeviceWithClaims(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1Claim, node1dev2Claim)

	pciDeviceClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovValidator := sriovNetworkDeviceValidator{
		claimCache: pciDeviceClaimCache,
		nodeCache:  nodeCache,
	}

	newObj := sriovDeviceEnabled.DeepCopy()
	newObj.Spec.NumVFs = 0
	err := sriovValidator.Update(nil, sriovDeviceEnabled, newObj)
	assert.Error(err, "expected validation to fail")
}

func Test_DisableSRIOVDeviceWithoutClaims(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset()

	pciDeviceClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovValidator := sriovNetworkDeviceValidator{
		claimCache: pciDeviceClaimCache,
		nodeCache:  nodeCache,
	}

	newObj := sriovDeviceEnabled.DeepCopy()
	newObj.Spec.NumVFs = 0
	err := sriovValidator.Update(nil, sriovDeviceEnabled, newObj)
	assert.NoError(err, "expected validation to pass")
}

func Test_DeleteSRIOVDeviceWithClaims(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node1dev1Claim, node1dev2Claim)

	pciDeviceClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovValidator := sriovNetworkDeviceValidator{
		claimCache: pciDeviceClaimCache,
		nodeCache:  nodeCache,
	}

	err := sriovValidator.Delete(nil, sriovDeviceEnabled)
	assert.Error(err, "expected validation to fail")
}

func Test_DeleteSRIOVDeviceWithoutClaims(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset()

	pciDeviceClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovValidator := sriovNetworkDeviceValidator{
		claimCache: pciDeviceClaimCache,
		nodeCache:  nodeCache,
	}

	err := sriovValidator.Delete(nil, sriovDeviceEnabled)
	assert.NoError(err, "expected validation to pass")
}

func Test_DeleteSRIOVDeviceWithClaimsOnDeletedNode(t *testing.T) {
	assert := require.New(t)
	fakeClient := fake.NewSimpleClientset(node2dev1Claim, node1dev1Claim, node1dev2Claim)

	pciDeviceClaimCache := fakeclients.PCIDeviceClaimsCache(fakeClient.DevicesV1beta1().PCIDeviceClaims)
	sriovValidator := sriovNetworkDeviceValidator{
		claimCache: pciDeviceClaimCache,
		nodeCache:  nodeCache,
	}

	err := sriovValidator.Delete(nil, sriovDeviceEnabledOnDeletedNode)
	assert.Error(err, "expected validation to pass")
}
