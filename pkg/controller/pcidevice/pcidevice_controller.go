package pcidevice

import (
	"context"
	"os"
	"time"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/pci"
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
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	// start goroutine to regularly reconcile the PCI Devices list
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Devices list")
			if err := handler.reconcilePCIDevices(hostname); err != nil {
				logrus.Errorf("PCI device reconciliation error: %v", err)
			}
		}
	}()
	return nil
}

func (h Handler) reconcilePCIDevices(hostname string) error {
	// List all PCI Devices on host
	busReader, err := pci.NewBusReader()
	if err != nil {
		return err
	}
	var pcidevices []*pci.PCI
	pcidevices, err = busReader.Read()
	if err != nil {
		return err
	}

	for _, dev := range pcidevices {
		if dev.ClassName == "NetworkEthernet" || dev.ClassName == "DisplayVGA" {
			name := v1beta1.PCIDeviceNameForHostname(dev, hostname)
			// Check if device is stored
			devCR, err := h.client.Get(name, metav1.GetOptions{})

			if err == nil {
				// Update the stored device
				devCR.Status.Update(dev, hostname) // update the in-memory CR with the current PCI info
				_, err = h.client.Update(devCR)
				if err != nil {
					logrus.Errorf("Failed to update %v: %s\n", devCR.Status.Address, err)
				}
				_, err = h.client.UpdateStatus(devCR)
				if err != nil {
					logrus.Errorf("(Resource exists) Failed to update status sub-resource: %s\n", err)
				}
			} else {
				logrus.Errorf("Failed to get %s: %s\n", name, err)

				// Create the PCIDevice CR if it doesn't exist
				var pdToCreate v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, hostname)
				logrus.Infof("Creating PCI Device: %s\n", err)
				_, err := h.client.Create(&pdToCreate)
				if err != nil {
					logrus.Errorf("Failed to create PCI Device: %s\n", err)
				}
			}
		}
	}
	// TODO loop through stored CRs and see if any need to be deleted
	return nil
}
