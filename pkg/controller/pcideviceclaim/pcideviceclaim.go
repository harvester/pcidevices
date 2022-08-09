package pcideviceclaim

import (
	"context"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
)

const (
	controllerName = "harvester-pcidevices-controller"
)

type Controller struct {
	PCIDeviceClaims ctl.PCIDeviceClaimController
}

func Register(
	ctx context.Context,
	pdcctl ctl.PCIDeviceClaimController,
) error {
	c := &Controller{
		PCIDeviceClaims: pdcctl,
	}
	pdcctl.OnChange(ctx, controllerName, c.OnChange)
	return nil
}

func (c *Controller) OnChange(key string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	logrus.Infof("PCI Device Claim %s has changed", pdc)
	return pdc, nil
}
