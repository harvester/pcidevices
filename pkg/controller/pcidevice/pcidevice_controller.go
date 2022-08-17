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
	controllerName  = "harvester-pcidevices-controller"
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
	pcidevicesCRs, err := h.client.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var actual map[string]int = make(map[string]int)
	var stored map[string]int = make(map[string]int)

	// Actual PCI Devices
	for i, dev := range pcidevices {
		actual[dev.Addr] = i
	}
	// Stored PCI Device CRs (Custom Resources) for this node
	if err != nil {
		return err
	}
	for i, devCR := range pcidevicesCRs.Items {
		// Only look at devices on _this_ node
		deviceOnThisNode := devCR.Status.NodeName == hostname
		if deviceOnThisNode {
			stored[devCR.Status.Address] = i
		}
	}
	// Diff with PCI Device CRDs on cluster (filtered by host)
	for addr := range actual {
		indexActual := actual[addr]
		dev := pcidevices[indexActual]
		indexStored, found := stored[addr]
		if found {
			// Update the PCIDevice CR since it exists
			devStored := pcidevicesCRs.Items[indexStored]
			name := devStored.ObjectMeta.Name
			devCR, err := h.client.Get(name, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Failed to get %s: %s\n", name, err)
			}
			devCR.Status.Update(dev) // update the in-memory CR with the current PCI info
			_, err = h.client.Update(devCR)
			if err != nil {
				logrus.Errorf("Failed to update %v: %s\n", devCR.Status.Address, err)
			}
			_, err = h.client.UpdateStatus(devCR)
			if err != nil {
				logrus.Errorf("Failed to update status sub-resource: %s\n", err)
			}
		} else {
			// Create the PCIDevice CR if it doesn't exist
			var pdToCreate v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, hostname)
			pdCreated, err := h.client.Create(&pdToCreate)
			if err != nil {
				logrus.Errorf("Failed to create PCI Device: %s\n", err)
			}
			pdCreated.Status.Update(dev)
			pdCreated.Status.NodeName = hostname
			_, err = h.client.UpdateStatus(pdCreated)
			if err != nil {
				logrus.Errorf("Failed to update status sub-resource: %s\n", err)
			}
		}
	}
	// TODO loop through stored CRs and see if any need to be deleted
	return nil
}
