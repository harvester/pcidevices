package nodes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/harvester/pcidevices/pkg/util/fakeclients"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	fakev1beta1 "github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
)

var (
	nodeWithGPU = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-with-gpu",
		},
	}

	nodeWithoutGPU = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-without-gpu",
			Labels: map[string]string{
				v1beta1.NvidiaDriverNeededKey: "true",
			},
		},
	}
	sriovGPUDevice1 = &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu1",
			Labels: map[string]string{
				v1beta1.NodeKeyName: nodeWithGPU.Name,
			},
		},
	}
)

func Test_checkAndUpdateNodeLabels(t *testing.T) {

	var tests = []struct {
		name        string
		nodeName    string
		expectLabel bool
	}{
		{
			name:        "expect to find label",
			nodeName:    nodeWithGPU.Name,
			expectLabel: true,
		},
		{
			name:        "expect to not find label",
			nodeName:    nodeWithoutGPU.Name,
			expectLabel: false,
		},
	}
	assert := require.New(t)
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	c := fake.NewSimpleClientset(nodeWithGPU, nodeWithoutGPU)
	nodeClient := fakeclients.NodeClient(c.CoreV1().Nodes)
	nodeCache := fakeclients.NodeCache(c.CoreV1().Nodes)
	v1beta1Client := fakev1beta1.NewSimpleClientset(sriovGPUDevice1)
	sriovGPUCache := fakeclients.SriovGPUDevicesCache(v1beta1Client.DevicesV1beta1().SRIOVGPUDevices)

	for _, v := range tests {
		err := checkAndUpdateNodeLabels(v.nodeName, nodeCache, nodeClient, sriovGPUCache)
		assert.NoError(err, "expected no error during reconcile of nodes")
		nodeObj, err := c.CoreV1().Nodes().Get(context.TODO(), v.nodeName, metav1.GetOptions{})
		assert.NoError(err, "expected no error while fetching node")
		if v.expectLabel {
			assert.Equal(nodeObj.Labels[v1beta1.NvidiaDriverNeededKey], "true", fmt.Sprintf("case: %s", v.name))
		} else {
			assert.Equal(nodeObj.Labels[v1beta1.NvidiaDriverNeededKey], "", fmt.Sprintf("case: %s", v.name))
		}
	}

}
