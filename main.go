package main

import (
	"context"
	"fmt"
	"github.com/harvester/pcidevices/pkg/webhook"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/controller/pcidevice"
	"github.com/harvester/pcidevices/pkg/controller/pcideviceclaim"
	"github.com/harvester/pcidevices/pkg/crd"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	VERSION        = "v0.0.2"
	controllerName = "harvester-pcideviceclaims-controller"
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
	factory, err := controller.NewSharedControllerFactoryFromConfig(cfg, Scheme)
	if err != nil {
		return err
	}
	opts := &generic.FactoryOptions{
		SharedControllerFactory: factory,
	}
	pdfactory, err := ctl.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("error building pcidevice controllers: %s", err.Error())
	}
	pdcfactory, err := ctl.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("error building pcideviceclaim controllers: %s", err.Error())
	}
	if err != nil {
		return err
	}
	registerControllers := func(ctx context.Context) {
		pdCtl := pdfactory.Devices().V1beta1().PCIDevice()
		logrus.Info("Starting PCI Devices controller")
		if err := pcidevice.Register(ctx, pdCtl); err != nil {
			logrus.Fatalf("failed to register PCI Devices Controller")
		}

		pdcCtl := pdcfactory.Devices().V1beta1().PCIDeviceClaim()
		logrus.Info("Starting PCI Device Claims Controller")
		if err = pcideviceclaim.Register(ctx, pdcCtl, pdCtl); err != nil {
			logrus.Fatalf("failed to register PCI Device Claims Controller")
		}
	}

	startAllControllers := func(ctx context.Context) {
		if err := start.All(ctx, 2, pdfactory, pdcfactory); err != nil {
			logrus.Fatalf("Error starting: %s", err.Error())
		}
	}

	registerControllers(ctx)
	startAllControllers(ctx)

	w := webhook.New(ctx, cfg)
	if err := w.ListenAndServe(); err != nil {
		logrus.Fatalf("Error starting webook: %v", err)
	}
<<<<<<< HEAD

=======
	
>>>>>>> ea17eac6c3783da6c8f2d5a2e7344eb25327539e
	<-ctx.Done()

	return nil
}
