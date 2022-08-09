package pcidevice

import (
	"context"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
)

const (
	controllerName = "harvester-pcidevices-controller"
)

type Controller struct {
	PCIDevices ctl.PCIDeviceController
}

func Register(
	ctx context.Context,
	pdctl ctl.PCIDeviceController,
) error {
	c := &Controller{
		PCIDevices: pdctl,
	}
	pdctl.OnChange(ctx, controllerName, c.OnChange)
	return nil
}

func (c *Controller) OnChange(key string, pd *v1beta1.PCIDevice) (*v1beta1.PCIDevice, error) {
	logrus.Infof("PCI Device %s has changed", pd)
	return pd, nil
}
