package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/start"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/config"
	"github.com/harvester/pcidevices/pkg/controller/nodecleanup"
	"github.com/harvester/pcidevices/pkg/crd"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
	"github.com/harvester/pcidevices/pkg/webhook"
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
)

// Declarations for Ginkgo DSL
var Fail = ginkgo.Fail
var Describe = ginkgo.Describe
var It = ginkgo.It
var By = ginkgo.By
var BeforeEach = ginkgo.BeforeEach
var AfterEach = ginkgo.AfterEach
var BeforeSuite = ginkgo.BeforeSuite
var AfterSuite = ginkgo.AfterSuite
var RunSpecs = ginkgo.RunSpecs
var GinkgoWriter = ginkgo.GinkgoWriter

// Declarations for Gomega Matchers
var RegisterFailHandler = gomega.RegisterFailHandler
var Equal = gomega.Equal
var Expect = gomega.Expect
var BeNil = gomega.BeNil
var HaveOccurred = gomega.HaveOccurred
var BeEmpty = gomega.BeEmpty
var Eventually = gomega.Eventually

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
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("..", "manifests")},
		},
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

	err = kubevirtv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = crd.Create(ctx, cfg)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	err = k8sClient.Create(ctx, defaultNS)
	Expect(err).NotTo(HaveOccurred())

	// start webhook //
	w := webhook.New(ctx, cfg)
	err = w.ListenAndServe()
	Expect(err).NotTo(HaveOccurred())

	sharedFactory, err := controller.NewSharedControllerFactoryFromConfig(cfg, scheme)
	Expect(err).NotTo(HaveOccurred())
	opts := &generic.FactoryOptions{
		SharedControllerFactory: sharedFactory,
	}

	factory, err := ctl.NewFactoryFromConfigWithOptions(cfg, opts)
	Expect(err).NotTo(HaveOccurred())

	coreFactory, err := core.NewFactoryFromConfigWithOptions(cfg, &core.FactoryOptions{
		SharedControllerFactory: sharedFactory,
	})

	Expect(err).NotTo(HaveOccurred())

	management := config.NewFactoryManager(factory, coreFactory, nil, nil, nil, nil)

	err = nodecleanup.Register(ctx, management)
	Expect(err).NotTo(HaveOccurred())
	err = start.All(ctx, 1, factory, coreFactory)
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
