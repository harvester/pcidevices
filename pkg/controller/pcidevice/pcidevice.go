package pcidevice

import (
	"context"
	"os"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/pci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	logrus.Info("Registering PCI Devices controller")
	c := &Controller{
		PCIDevices: pdctl,
	}
	pdctl.OnChange(ctx, controllerName, c.OnChange)
	// HACK: Call OnChange once just to get the PCI Devices list built out
	c.OnChange("initial run", nil)
	return nil
}

func (c *Controller) OnChange(key string, pd *v1beta1.PCIDevice) (*v1beta1.PCIDevice, error) {
	if key == "initial run" {
		logrus.Infof("PCI Device daemon is starting")
	} else {
		logrus.Infof("PCI Device %s has changed", &pd.ObjectMeta.Name)
	}
	// List all PCI Devices on host
	busReader, err := pci.NewBusReader()
	if err != nil {
		return nil, err
	}
	var pcidevices []*pci.PCI
	pcidevices, err = busReader.Read()
	if err != nil {
		return nil, err
	}
	client := c.PCIDevices
	pcidevicesCRs, err := client.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var actual map[string]int = make(map[string]int)
	var stored map[string]int = make(map[string]int)

	// Actual PCI Devices
	for i, dev := range pcidevices {
		actual[dev.Addr] = i
	}
	// Stored PCI Device CRs (Custom Resources) for this node
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	for i, devCR := range pcidevicesCRs.Items {
		// Only look at devices on _this_ node
		if devCR.Status.NodeName == hostname {
			stored[devCR.Status.Address] = i
		}
	}
	// Diff with PCI Device CRDs on cluster (filtered by host)
	var index int
	var found bool
	for addr := range actual {
		index, found = stored[addr]
		if !found {
			// Create a stored CR for this PCI device
			var dev *pci.PCI = pcidevices[index]
			var pcidevice v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, hostname)
			_, err := client.Create(&pcidevice)
			if err != nil {
				logrus.Errorf("Failed to create PCI Device: %s\n", err)
			}
		}
	}

	// TODO Delete stored CRs that are no longer in actual
	return pd, nil
}
