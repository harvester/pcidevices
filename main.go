package main

import (
	"fmt"
	"os"

	harvesternetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/apis/network.harvesterhci.io/v1beta1"
	ctlnetwork "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io"
	"github.com/rancher/lasso/pkg/controller"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/controller/nodecleanup"
	"github.com/harvester/pcidevices/pkg/controller/pcidevice"
	"github.com/harvester/pcidevices/pkg/controller/pcideviceclaim"
	"github.com/harvester/pcidevices/pkg/crd"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
	"github.com/harvester/pcidevices/pkg/webhook"
)

const (
	VERSION        = "v0.1.0"
	controllerName = "pcidevices-controller"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		v1beta1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(Scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(Scheme))
	utilruntime.Must(harvesternetworkv1beta1.AddToScheme(Scheme))
	if debug := os.Getenv("DEBUG_LOGGING"); debug == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func main() {
	// set up the kubeconfig and other args
	var kubeConfig string
	app := cli.NewApp()
	app.Name = controllerName
	app.Version = VERSION
	app.Usage = "Harvester PCI Devices Controller, to discover PCI devices on the nodes of a cluster. Also manages PCI Device Claims, for use in PCI passthrough."
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "kubeconfig",
			EnvVars:     []string{"KUBECONFIG"},
			Destination: &kubeConfig,
			Usage:       "Kube config for accessing k8s cluster",
		},
	}

	app.Action = func(c *cli.Context) error {
		return run(kubeConfig)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(kubeConfig string) error {
	ctx := signals.SetupSignalContext()

	var cfg *rest.Config
	cfg, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to find kubeconfig: %v", err)
	}

	// Create CRDs
	err = crd.Create(ctx, cfg)
	if err != nil {
		return err
	}

	// Register scheme with the shared factory controller
	sharedFactory, err := controller.NewSharedControllerFactoryFromConfig(cfg, Scheme)
	if err != nil {
		return err
	}
	opts := &generic.FactoryOptions{
		SharedControllerFactory: sharedFactory,
	}
	pciFactory, err := ctl.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("error building pcidevice controllers: %s", err.Error())
	}

	workqueue.DefaultControllerRateLimiter()

	coreFactory, err := ctlcore.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("error building core controllers: %v", err)
	}

	networkFactory, err := ctlnetwork.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("error building network controllers: %v", err)
	}
	pdCtl := pciFactory.Devices().V1beta1().PCIDevice()
	pdcCtl := pciFactory.Devices().V1beta1().PCIDeviceClaim()
	nodeCtl := coreFactory.Core().V1().Node()
	nodeName := os.Getenv("NODE_NAME")

	if err := pcideviceclaim.Register(ctx, pdcCtl, pdCtl, nodeName); err != nil {
		return fmt.Errorf("error registering pcidevicesclaim controller: %v", err)
	}

	if err := nodecleanup.Register(ctx, pdcCtl, pdCtl, nodeCtl); err != nil {
		logrus.Fatalf("failed to register node cleanup controller: %v", err)
	}

	eg, egctx := errgroup.WithContext(ctx)
	w := webhook.New(egctx, cfg)

	eg.Go(func() error {
		err := w.ListenAndServe()
		if err != nil {
			logrus.Errorf("Error starting webook: %v", err)
		}
		return err
	})

	eg.Go(func() error {
		return pcidevice.Register(egctx, pdCtl, coreFactory, networkFactory)
	})

	if err := start.All(ctx, 2, coreFactory, networkFactory, pciFactory); err != nil {
		return fmt.Errorf("error starting factories: %v", err)
	}

	return eg.Wait()
}
