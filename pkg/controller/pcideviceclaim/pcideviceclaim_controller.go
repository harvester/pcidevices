package pcideviceclaim

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	controllerName  = "harvester-pcidevices-controller"
	reconcilePeriod = time.Second * 20
)

type Controller struct {
	PCIDeviceClaims ctl.PCIDeviceClaimController
}

type Handler struct {
	pdcClient ctl.PCIDeviceClaimClient
	pdClient  ctl.PCIDeviceClient
}

func Register(
	ctx context.Context,
	pdc ctl.PCIDeviceClaimClient,
	pd ctl.PCIDeviceClient,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	handler := &Handler{
		pdcClient: pdc,
		pdClient:  pd,
	}
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	// start goroutine to regularly reconcile the PCI Device Claims' status with their spec
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Device Claims list")
			if err := handler.reconcilePCIDeviceClaims(hostname); err != nil {
				logrus.Errorf("PCI Device Claim reconciliation error")
			}
		}
	}()
	return nil
}

func (c *Controller) OnChange(key string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	logrus.Infof("PCI Device Claim %s has changed", pdc.Name)
	return pdc, nil
}

func (h Handler) reconcilePCIDeviceClaims(hostname string) error {
	// Get all PCI Device Claims
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	// Get all PCI Devices
	pds, err := h.pdClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	// Build up map[node-addr]=>name
	var pdNames map[string]string = make(map[string]string)
	for _, pd := range pds.Items {
		nodeAddr := fmt.Sprintf(
			"%s-%s", pd.Status.NodeName, pd.Status.Address,
		)
		pdNames[nodeAddr] = pd.ObjectMeta.Name
	}

	// Get those PCI Device Claims for this node
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName == hostname {
			if !pdc.Status.PassthroughEnabled {
				logrus.Infof("Attempting to enable passthrough")
				// TEMPORARY, JUST FOR UI DEVELOPMENT
				pdc.Status.PassthroughEnabled = true
				// Get PCIDevice for the PCIDeviceClaim
				name := pdNames[pdc.Spec.NodeAddr()]
				pd, err := h.pdClient.Get(name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				pdc.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
				_, err = h.pdcClient.Update(&pdc)
				if err != nil {
					return err
				}
				_, err = h.pdcClient.UpdateStatus(&pdc)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
