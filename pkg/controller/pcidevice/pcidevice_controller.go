package pcidevice

import (
	"context"
	"os"
	"time"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	reconcilePeriod = time.Second * 20
)

type Handler struct {
	client ctl.PCIDeviceClient
}

func Register(
	ctx context.Context,
	pd ctl.PCIDeviceClient,
) error {
	logrus.Info("Registering PCI Devices controller")
	handler := &Handler{
		client: pd,
	}
	nodename := os.Getenv("NODE_NAME")
	// start goroutine to regularly reconcile the PCI Devices list
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Devices list")
			if err := handler.reconcilePCIDevices(nodename); err != nil {
				logrus.Errorf("PCI device reconciliation error: %v", err)
			}
		}
	}()
	return nil
}

func (h Handler) reconcilePCIDevices(nodename string) error {
	// List all PCI Devices on host
	pci, err := ghw.PCI()
	if err != nil {
		return err
	}

	var setOfRealPCIAddrs map[string]bool = make(map[string]bool)
	for _, dev := range pci.Devices {
		setOfRealPCIAddrs[dev.Address] = true
		name := v1beta1.PCIDeviceNameForHostname(dev, nodename)
		// Check if device is stored
		_, err := h.client.Get(name, metav1.GetOptions{})

		if err != nil {
			logrus.Errorf("Failed to get %s: %s\n", name, err)

			// Create the PCIDevice CR if it doesn't exist
			var pdToCreate v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, nodename)
			logrus.Infof("Creating PCI Device: %s\n", err)
			pdToCreate.Labels["nodename"] = nodename // label
			_, err := h.client.Create(&pdToCreate)
			if err != nil {
				logrus.Errorf("Failed to create PCI Device: %s\n", err)
			}
		}
		// Update the stored device
		devCR, err := h.client.Get(name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get %s: %s\n", name, err)
		}
		devCopy := devCR.DeepCopy()
		devCopy.Status.Update(dev, nodename) // update the in-memory CR with the current PCI info
		_, err = h.client.Update(devCopy)
		if err != nil {
			logrus.Errorf("Failed to update %v: %s\n", devCopy.Status.Address, err)
		}
		_, err = h.client.UpdateStatus(devCopy)
		if err != nil {
			logrus.Errorf("(Resource exists) Failed to update status sub-resource: %s\n", err)
		}
	}
	return nil
}
