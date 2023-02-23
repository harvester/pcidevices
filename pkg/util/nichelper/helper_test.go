package nichelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harvester/harvester-network-controller/pkg/apis/network.harvesterhci.io/v1beta1"
	fakenetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned/fake"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/option"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	vlanConfigAllNodes = &v1beta1.VlanConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "all-nodes",
			Annotations: map[string]string{
				"network.harvesterhci.io/matched-nodes": "[\"node1\",\"node2\"]",
			},
		},
		Spec: v1beta1.VlanConfigSpec{
			ClusterNetwork: "workload",
			Uplink: v1beta1.Uplink{
				NICs: []string{"eno49"},
			},
		},
	}

	vlanConfigSpecificNodes = &v1beta1.VlanConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2-match",
			Annotations: map[string]string{
				"network.harvesterhci.io/matched-nodes": "[\"node2\"]",
			},
		},
		Spec: v1beta1.VlanConfigSpec{
			ClusterNetwork: "workload",
			Uplink: v1beta1.Uplink{
				NICs: []string{"eno50"},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "node2",
			},
		},
	}

	node1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"kubernetes.io/hostname": "node1",
			},
		},
	}

	node2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				"kubernetes.io/hostname": "node2",
			},
		},
	}

	mockPath string
)

func TestMain(m *testing.M) {
	mockPath = os.Getenv("UMOCKDEV_DIR")
	if mockPath != "" {
		defaultDevicePath = filepath.Join(mockPath, defaultDevicePath)
	}
	exit := m.Run()
	os.Exit(exit)
}

func Test_MatchAllNodes(t *testing.T) {
	assert := require.New(t)

	vlanConfigClient := fakenetworkclient.NewSimpleClientset(vlanConfigAllNodes)
	k8sClient := k8sfake.NewSimpleClientset(node1, node2)

	vlanConfigCache := fakeclients.VlanConfigCache(vlanConfigClient.NetworkV1beta1().VlanConfigs)
	nodeCache := fakeclients.NodeCache(k8sClient.CoreV1().Nodes)

	nics, err := identifyClusterNetworks("node1", nodeCache, vlanConfigCache)
	assert.NoError(err, "expected no error during call to identify cluster networks")
	assert.Len(nics, 1, "expected to find one nic")
}

func Test_NoMatchSpecificNode(t *testing.T) {
	assert := require.New(t)

	vlanConfigClient := fakenetworkclient.NewSimpleClientset(vlanConfigSpecificNodes)
	k8sClient := k8sfake.NewSimpleClientset(node1, node2)

	vlanConfigCache := fakeclients.VlanConfigCache(vlanConfigClient.NetworkV1beta1().VlanConfigs)
	nodeCache := fakeclients.NodeCache(k8sClient.CoreV1().Nodes)

	nics, err := identifyClusterNetworks("node1", nodeCache, vlanConfigCache)
	assert.NoError(err, "expected no error during call to identify cluster networks")
	assert.Len(nics, 0, "expected to find one nic")
}

func Test_MatchSpecificNode(t *testing.T) {
	assert := require.New(t)

	vlanConfigClient := fakenetworkclient.NewSimpleClientset(vlanConfigSpecificNodes)
	k8sClient := k8sfake.NewSimpleClientset(node1, node2)

	vlanConfigCache := fakeclients.VlanConfigCache(vlanConfigClient.NetworkV1beta1().VlanConfigs)
	nodeCache := fakeclients.NodeCache(k8sClient.CoreV1().Nodes)

	nics, err := identifyClusterNetworks("node2", nodeCache, vlanConfigCache)
	assert.NoError(err, "expected no error during call to identify cluster networks")
	assert.Len(nics, 1, "expected to find one nic")
}

func Test_GenerateSriovNics(t *testing.T) {
	nodeName := "fake"
	assert := require.New(t)

	nics, err := ghw.Network(&option.Option{
		Chroot: &mockPath,
	})

	assert.NoError(err)

	generatedObjs, err := generateSRIOVDeviceObjects(nodeName, nics, nil)
	assert.NoError(err, "expected no error during generation of sriov device objects")
	assert.Len(generatedObjs, 2, "expected to find 2 sriov devices")

}

func Test_ConfigureAndVerifyVF(t *testing.T) {
	assert := require.New(t)
	pfAddress := "0000:04:00.0"

	err := ConfigureVF(pfAddress, 4)
	assert.NoError(err, "expected no error during VF configuration")
	count, err := CurrentVFConfigured(pfAddress)
	assert.NoError(err, "expected no error during PF lookup")
	assert.Equal(4, count, "expected to find 4")
}

func Test_IdentifyManagementNics(t *testing.T) {
	assert := require.New(t)
	err := os.Setenv("GHW_CHROOT", mockPath)
	assert.NoError(err)
	_, err = IdentifyManagementNics()
	assert.NoError(err)
	err = os.Unsetenv("GHW_CHROOT")
	assert.NoError(err)

}
