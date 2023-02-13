package pcidevice

import (
	"context"
	"testing"

	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
	"github.com/jaypipes/ghw"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultPCIDeviceSnapshot = "../../../tests/snapshots/linux-amd64-e147d239df014921c6cbb49fbc3d6c41.tar.gz"
)

func Test_reconcilePCIDevices(t *testing.T) {
	assert := require.New(t)
	client := fake.NewSimpleClientset()

	pci, err := ghw.PCI(ghw.WithSnapshot(ghw.SnapshotOptions{
		Path: defaultPCIDeviceSnapshot,
	}))
	assert.NoError(err, "expected no error during snapshot loading")

	h := Handler{
		client:        fakeclients.PCIDevicesClient(client.DevicesV1beta1().PCIDevices),
		pci:           pci,
		skipAddresses: []string{"0000:04:00.1"}, //address of eno5 interface in the snapshot
	}

	err = h.reconcilePCIDevices("TEST_NODE")
	assert.NoError(err, "expected no error during pcidevice reconcile")
	// check if GPU device is created. Default Name is TEST_NODE-000008000
	gpuDevice, err := client.DevicesV1beta1().PCIDevices().Get(context.TODO(), "TEST_NODE-000008000", metav1.GetOptions{})
	assert.NoError(err, "expected no error while listing GPU")
	assert.NotNil(gpuDevice, "expected to find a GPU device")
	_, err = client.DevicesV1beta1().PCIDevices().Get(context.TODO(), "TEST_NODE-000004001", metav1.GetOptions{})
	assert.True(apierrors.IsNotFound(err), "expected to not find the pci address for 000004001")
	t.Log(gpuDevice.Status)
}
