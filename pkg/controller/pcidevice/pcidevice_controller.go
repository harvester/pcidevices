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
	pciDeviceClient ctl.PCIDeviceClient
}

func Register(
	ctx context.Context,
	pd ctl.PCIDeviceClient,
) error {
	logrus.Info("Registering PCI Devices controller")
	handler := &Handler{
		pciDeviceClient: pd,
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

type pair struct {
	v uint16 // vendorId
	d uint16 // deviceID
}

// TODO make this into a configMap
var supportedDevices map[pair]bool = map[pair]bool{
	{v: 0x8086, d: 0x1521}: true, // Intel Ethernet Controller I350
	{v: 0x8086, d: 0x0d4c}: true, // Intel Ethernet Controller I219-LM
	{v: 0x10de, d: 0x1c02}: true, // NVIDIA GeForce GTX 1060
	{v: 0x10de, d: 0x2184}: true, // NVIDIA GeForce GTX 1660
}

func deviceIsSupported(vendorId uint16, deviceId uint16) bool {
	p := pair{vendorId, deviceId}

	val, ok := supportedDevices[p]
	if !ok {
		return false
	}
	return val
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
		if deviceIsSupported(dev.Vendor, dev.Device) {
			actual[dev.Addr] = i
		}
	}
	// Stored PCI Device CRs (Custom Resources) for this node
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
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
		supported := deviceIsSupported(dev.Vendor, dev.Device)
		indexStored, found := stored[addr]
		if found {
			// Update the PCIDevice CR since it exists
			devCR := pcidevicesCRs.Items[indexStored]
			devCR.Status.Update(dev) // update the in-memory CR with the current PCI info
			h.pciDeviceClient.Update(&devCR)
		} else {
			if supported {
				// Create the PCIDevice CR if it doesn't exist
				var pcidevice v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, hostname)
				_, err := h.pciDeviceClient.Create(&pcidevice)
				if err != nil {
					logrus.Errorf("Failed to create PCI Device: %s\n", err)
				}
			}
		}
	}
	// TODO loop through stored CRs and see if any need to be deleted
	return nil
}
