package sriovdevice

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	fakenetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	fakeclient "github.com/harvester/pcidevices/pkg/util/fakeclients"
	"github.com/harvester/pcidevices/pkg/util/nichelper"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var (
	mockPath      string
	deviceEnabled = &v1beta1.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-eno49",
		},
		Spec: v1beta1.SRIOVNetworkDeviceSpec{
			NodeName: "mock",
			Address:  "0000:04:00.0",
			NumVFs:   4,
		},
	}

	deviceDisabled = &v1beta1.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-eno50",
		},
		Spec: v1beta1.SRIOVNetworkDeviceSpec{
			NodeName: "mock",
			Address:  "0000:04:00.0",
			NumVFs:   0,
		},
	}
)

func TestMain(m *testing.M) {
	mockPath = os.Getenv("UMOCKDEV_DIR")
	if mockPath != "" {
		nichelper.OverrideDefaultSysPath(filepath.Join(mockPath, nichelper.GetDefaultSysPath()))
	}
	exit := m.Run()
	os.Exit(exit)
}

func Test_ReconcileEnable(t *testing.T) {
	fakeDevicesClient := fake.NewSimpleClientset(deviceEnabled)
	h := &Handler{
		ctx:         context.TODO(),
		sriovClient: fakeclient.SriovDevicesClient(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices),
		sriovCache:  fakeclient.SriovDevicesCache(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices),
		nodeName:    "mock",
	}

	assert := require.New(t)
	retDevice, err := h.reconcileSriovDevice("", deviceEnabled)
	t.Log(retDevice)
	assert.NoError(err, "expected no error during reconcile of sriov-device")
	assert.Equal(v1beta1.DeviceEnabled, retDevice.Status.Status, "expected to find device to be enabled")
}

func Test_ReconcileDisable(t *testing.T) {
	fakeDevicesClient := fake.NewSimpleClientset(deviceDisabled)
	h := &Handler{
		ctx:         context.TODO(),
		sriovClient: fakeclient.SriovDevicesClient(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices),
		sriovCache:  fakeclient.SriovDevicesCache(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices),
		nodeName:    "mock",
	}

	assert := require.New(t)
	retDevice, err := h.reconcileSriovDevice("", deviceDisabled)
	assert.NoError(err, "expected no error during reconcile of sriov-device")
	assert.Equal(v1beta1.DeviceDisabled, retDevice.Status.Status, "expected to find device to be enabled")
}

func Test_SetupSriovDevices(t *testing.T) {
	fakeDevicesClient := fake.NewSimpleClientset()
	vlanConfigClient := fakenetworkclient.NewSimpleClientset()
	k8sClient := k8sfake.NewSimpleClientset()

	sriovCache := fakeclient.SriovDevicesCache(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices)
	h := &Handler{
		ctx:             context.TODO(),
		sriovClient:     fakeclient.SriovDevicesClient(fakeDevicesClient.DevicesV1beta1().SRIOVNetworkDevices),
		sriovCache:      sriovCache,
		vlanConfigCache: fakeclient.VlanConfigCache(vlanConfigClient.NetworkV1beta1().VlanConfigs),
		nodeCache:       fakeclient.NodeCache(k8sClient.CoreV1().Nodes),
		nodeName:        "mock",
	}

	assert := require.New(t)
	err := os.Setenv("GHW_CHROOT", mockPath)
	assert.NoError(err)

	err = h.SetupSriovDevices()
	assert.NoError(err, "expected no error during setup of sriov devices")
	devices, err := sriovCache.List(labels.NewSelector())
	assert.NoError(err, "expected no error while listing devices")
	assert.Len(devices, 2, "expected to find 2 devices generated from the mock data")
	err = os.Unsetenv("GHW_CHROOT")
	assert.NoError(err, "expect no error while unsetting GHW_CHROOT")

}
