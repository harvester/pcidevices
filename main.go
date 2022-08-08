package main

import (
	"context"

	"github.com/harvester/pcidevices/pkg/controller/pcidevice"
	"github.com/rancher/wrangler/pkg/signals"
)

func main() {
	ctx := signals.SetupSignalHandler(context.Background())
	pcidevices, err := pcidevice.NewFactoryFromConfig()
	// PCIDevices controller
	//pcidevice.Register(ctx, nil)
}
