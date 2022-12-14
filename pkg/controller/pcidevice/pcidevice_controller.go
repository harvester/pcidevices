package pcidevice

import (
	"context"
	"os"
	"time"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	commonLabels := map[string]string{"nodename": nodename} // label
	var setOfRealPCIAddrs map[string]bool = make(map[string]bool)
	for _, dev := range pci.Devices {
		setOfRealPCIAddrs[dev.Address] = true
		name := v1beta1.PCIDeviceNameForHostname(dev, nodename)
		// Check if device is stored
		var err error
		var devCR *v1beta1.PCIDevice
		devCR, err = h.client.Get(name, metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("[PCIDeviceController] Device %s does not exist", name)

				// Create the PCIDevice CR if it doesn't exist
				var pdToCreate v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, nodename)
				logrus.Infof("Creating PCI Device: %s\n", err)
				pdToCreate.Labels = commonLabels
				devCR, err = h.client.Create(&pdToCreate)
				if err != nil {
					logrus.Errorf("[PCIDeviceController] Failed to create PCI Device: %v", err)
					return err
				}
			} else {
				logrus.Errorf("[PCIDeviceController] error fetching device %s: %v", name, err)
				return err
			}

		}

		devCopy := devCR.DeepCopy()
		// Update only modifies the status, no need to update the main object
		devCopy.Status.Update(dev, nodename) // update the in-memory CR with the current PCI info
		_, err = h.client.UpdateStatus(devCopy)
		if err != nil {
			logrus.Errorf("[PCIDeviceController] Failed to update status sub-resource: %v", err)
			return err
		}
	}

	// remove non-existent devices
	selector := labels.SelectorFromValidatedSet(commonLabels)

	pdList, err := h.client.List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		logrus.Errorf("[PCIDeviceController] error listing devices for node %s: %v", nodename, err)
		return err
	}

	var deleteList []v1beta1.PCIDevice

	for _, v := range pdList.Items {
		if ok := setOfRealPCIAddrs[v.Status.Address]; !ok {
			deleteList = append(deleteList, v)
		}
	}

	for _, v := range deleteList {
		if err := h.client.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("[PCIDeviceController] Faield to delete non existent device: %s on node %s", v.Name, v.Status.NodeName)
			return err
		}
	}

	return nil
}
