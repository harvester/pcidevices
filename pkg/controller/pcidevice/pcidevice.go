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
	reconcilePeriod = time.Second * 10
)

type Handler struct {
	pciDeviceClient ctl.PCIDeviceClient
}

func Register(
	ctx context.Context,
	pciDeviceClient ctl.PCIDeviceClient,
) error {
	logrus.Info("Registering PCI Devices controller")
	handler := &Handler{
		pciDeviceClient,
	}
	// start goroutine to regularly reconcile the PCI Devices list
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Devices list")
			if err := handler.reconcilePCIDevices(); err != nil {
				logrus.Errorf("PCI device reconciliation error: %v", err)
			}
		}
	}()
	return nil
}

func (h Handler) reconcilePCIDevices() error {
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
	pcidevicesCRs, err := h.pciDeviceClient.List(metav1.ListOptions{})
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
	hostname, err := os.Hostname()
	if err != nil {
		return err
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
			_, err := h.pciDeviceClient.Create(&pcidevice)
			if err != nil {
				logrus.Errorf("Failed to create PCI Device: %s\n", err)
			}
		}
	}
	return nil
}
