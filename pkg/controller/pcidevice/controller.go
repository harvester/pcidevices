package pcidevice

import (
	"context"

	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type Controller struct {
	PCIDevices ctl.PCIDeviceController
}

func Register(
	ctx context.Context,
	pdctl ctl.PCIDeviceController
) error {
	controller := &Controller{
		PCIDevices: pdctl,
	}
}
