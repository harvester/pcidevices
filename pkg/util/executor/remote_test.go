package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	envCluster "github.com/harvester/harvester/tests/framework/cluster"
	"github.com/harvester/harvester/tests/framework/env"
	"github.com/rancher/wrangler/pkg/clients"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	cfg    *rest.Config
	client kubernetes.Interface
	rs     = runtime.NewScheme()
)

const (
	defaultNS = "kube-system"
	defaultDS = "kube-proxy"
)

func TestMain(t *testing.M) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	lw := log.Writer()
	clientcmd, cluster, err := setupEnvironment(log.Writer())
	if err != nil {
		log.Fatalf("error starting cluster: %v", err)
	}

	cfg, err = clientcmd.ClientConfig()
	if err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error fetching config %v", err)
	}

	err = corev1.AddToScheme(rs)
	if err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error generating kubernetes client %v", err)
	}

	err = appsv1.AddToScheme(rs)
	if err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error generating kubernetes client %v", err)
	}
	client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error generating kubernetes client %v", err)
	}

	wranglerClients, err := clients.New(clientcmd, nil)
	if err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error generating kubernetes client %v", err)
	}

	client = wranglerClients.K8s
	if err := waitForKubeProxy(lw); err != nil {
		cluster.Cleanup(lw)
		log.Fatalf("error querying ds: %v", err)
	}

	code := t.Run()
	cluster.Cleanup(lw)
	os.Exit(code)

}

func Test_RemoteCommandExecutor(t *testing.T) {
	assert := require.New(t)
	// execute command on kube-proxy pod on first node in the cluster
	pods, err := client.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "k8s-app=kube-proxy",
	})
	assert.NoError(err, "expected no error while listing kube-proxy pods")
	r, err := NewRemoteCommandExecutor(context.TODO(), cfg, &pods.Items[0])
	assert.NoError(err)
	out, err := r.Run("echo", []string{"-n", "message"})
	assert.NoError(err, string(out))
	assert.Equal(string(out), "message")
}

func setupEnvironment(output io.Writer) (clientcmd.ClientConfig, envCluster.Cluster, error) {
	var cluster envCluster.Cluster
	if env.IsUsingExistingCluster() {
		cluster = envCluster.GetExistCluster()
	} else {
		cluster = envCluster.NewLocalCluster()
	}

	err := cluster.Startup(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to startup the local cluster %s, %v", cluster, err)
	}

	KubeClientConfig, err := envCluster.GetConfig()
	return KubeClientConfig, cluster, err
}

func waitForKubeProxy(output io.Writer) error {
	for {
		ds, err := client.AppsV1().DaemonSets(defaultNS).Get(context.TODO(), defaultDS, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error fetching daemonset :%v", err)
		}
		if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
			return nil
		}
		if _, err := output.Write([]byte("waiting for kube-proxy daemonset to be ready\n")); err != nil {
			return err
		}

		time.Sleep(10 * time.Second)
	}
}
