package integration

import (
	"context"
	deviceClient "github.com/harvester/pcidevices/pkg/clientset/versioned"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/webhook"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coreClients "github.com/rancher/wrangler/pkg/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	scheme    = runtime.NewScheme()
	ctx       context.Context
	cancel    context.CancelFunc
	defaultNS = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "harvester-system",
		},
	}
	dc *deviceClient.Clientset
	cc *coreClients.Clients
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Controller Suite",
	)
}

var _ = BeforeSuite(func() {
	var err error
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "manifests", "crds")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = apiregistrationv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	dc, err = deviceClient.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	cc, err = coreClients.NewFromConfig(cfg, nil)
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	err = k8sClient.Create(ctx, defaultNS)
	Expect(err).NotTo(HaveOccurred())

	// start webhook //
	w := webhook.New(ctx, cfg)
	err = w.ListenAndServe()
	Expect(err).NotTo(HaveOccurred())

	// wait before running tests
	time.Sleep(30 * time.Second)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
