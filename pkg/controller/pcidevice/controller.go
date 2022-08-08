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

type Handler struct {
	client ctl.PCIDeviceClient
	cache  ctl.PCIDeviceCache
}

func Register(
	ctx context.Context,
	pdctl ctl.PCIDeviceController,
) error {
	handler := &Handler{}
	pdctl.OnChange(ctx, controllerName, handler.OnChange)
	return nil
}

func (h Handler) OnChange(key string, pd *v1beta1.PCIDevice) (*v1beta1.PCIDevice, error) {
	logrus.Infof("PCI Device %s has changed", pd)
	return pd, nil
}
