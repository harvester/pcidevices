package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/harvester/pcidevices/pkg/controller/pcidevice"
	"github.com/harvester/pcidevices/pkg/controller/pcideviceclaim"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
)

const VERSION = "v0.0.1-dev"

func main() {
	var kubeConfig string
	app := cli.NewApp()
	app.Name = "harvester-pcidevices-controller"
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

	cfg, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to find kubeconfig: %v", err)
	}
	pdfactory, err := ctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("error building pcidevice controllers: %s", err.Error())
	}
	if err != nil {
		return err
	}
	start := func(ctx context.Context) {
		pds := pdfactory.Devices().V1beta1().PCIDevice()
		pdcs := pdfactory.Devices().V1beta1().PCIDeviceClaim()
		logrus.Info("Starting PCI Devices controller")
		if err := pcidevice.Register(ctx, pds); err != nil {
			logrus.Fatalf("failed to register PCI Devices Controller")
		}
		logrus.Info("Starting PCI Device Claims controller")
		if err := pcideviceclaim.Register(ctx, pdcs); err != nil {
			logrus.Fatalf("failed to register PCI Device Claims Controller")
		}
	}

	start(ctx)
	<-ctx.Done()

	return nil
}
