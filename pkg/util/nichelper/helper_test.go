package nichelper

import (
	"testing"

	"github.com/harvester/harvester-network-controller/pkg/apis/network.harvesterhci.io/v1beta1"
	fakenetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned/fake"
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
)

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
