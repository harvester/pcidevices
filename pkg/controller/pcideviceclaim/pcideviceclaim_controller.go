package pcideviceclaim

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/kmodule"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	reconcilePeriod = time.Second * 20
)

type Controller struct {
	PCIDeviceClaims v1beta1gen.PCIDeviceClaimController
}

type Handler struct {
	pdcClient v1beta1gen.PCIDeviceClaimClient
	pdClient  v1beta1gen.PCIDeviceClient
}

func Register(
	ctx context.Context,
	pdcClient v1beta1gen.PCIDeviceClaimController,
	pdClient v1beta1gen.PCIDeviceController,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	handler := &Handler{
		pdcClient: pdcClient,
		pdClient:  pdClient,
	}
	nodename := os.Getenv("NODE_NAME")
	// start goroutine to regularly reconcile the PCI Device Claims' status with their spec
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Device Claims list")
			if err := handler.reconcilePCIDeviceClaims(nodename); err != nil {
				logrus.Errorf("PCI Device Claim reconciliation error: %v", err)
			}
		}
	}()
	return nil
}

func loadVfioDrivers() {
	for _, driver := range []string{"vfio-pci", "vfio_iommu_type1"} {
		if err := kmodule.Probe(driver, ""); err != nil {
			logrus.Error(err)
		}
	}
}

// Enabling passthrough for a PCI Device requires two steps:
// 1. Bind the device to the vfio-pci driver in the host
// 2. Permit the device in KubeVirt's Config in the cluster
func enablePassthrough(vendorId string, deviceId string, resourceName string) error {
	// 1. Bind the device to the vfio-pci driver in the host
	var id string = fmt.Sprintf("%s %s", vendorId, deviceId)

	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/new_id", os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = file.WriteString(id)
	if err != nil {
		return err
	}
	file.Close()

	// 2. Permit the device in KubeVirt's Config in the cluster
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		logrus.Fatalf("cannot obtain KubeVirt client: %v\n", err)
	}
	ns := "harvester-system"
	cr := "kubevirt"
	kubevirt, err := virtClient.KubeVirt(ns).Get(cr, &v1.GetOptions{})
	if err != nil {
		logrus.Fatalf("cannot obtain KubeVirt CR: %v\n", err)
	}
	if kubevirt.Spec.Configuration.PermittedHostDevices == nil {
		kubevirt.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			PciHostDevices: []kubevirtv1.PciHostDevice{},
		}
	}
	permittedPCIDevices := kubevirt.Spec.Configuration.PermittedHostDevices.PciHostDevices
	// check if device is currently permitted
	var devPermitted bool = false
	for _, permittedPCIDev := range permittedPCIDevices {
		if permittedPCIDev.ResourceName == resourceName {
			devPermitted = true
		}
	}
	if !devPermitted {
		devToPermit := kubevirtv1.PciHostDevice{
			PCIVendorSelector:        fmt.Sprintf("%s:%s", vendorId, deviceId),
			ResourceName:             resourceName,
			ExternalResourceProvider: false,
		}
		kubevirt.Spec.Configuration.PermittedHostDevices.PciHostDevices = append(permittedPCIDevices, devToPermit)
		_, err := virtClient.KubeVirt(ns).Update(kubevirt)
		if err != nil {
			logrus.Fatalf("Failed to update kubevirt CR: %s", err)
		}
	}

	return nil
}

func unbindPCIDeviceFromDriver(addr string, driver string) error {
	path := fmt.Sprintf("/sys/bus/pci/drivers/%s/unbind", driver)
	file, err := os.OpenFile(path, os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = file.WriteString(addr)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func unbindPCIDeviceFromVfioPCIDriver(addr string) error {
	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/unbind", os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = file.WriteString(addr)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func addHostDeviceToKubeVirtAllowList(pd *v1beta1.PCIDevice) {
	// Get KubeVirt CR
	// Get PermittedHostDevices
	// If pd is not in PermittedHostDevices,
	// then add it and update the CR
}

func (h Handler) reconcilePCIDeviceClaims(hostname string) error {
	// Get all PCI Device Claims
	var pdcNames map[string]string = make(map[string]string)
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pdc := range pdcs.Items {
		// Build up mapping
		nodeAddr := fmt.Sprintf(
			"%s-%s", pdc.Spec.NodeName, pdc.Spec.Address,
		)
		pdcNames[nodeAddr] = pdc.Name
	}
	// Get all PCI Devices
	pds, err := h.pdClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	// Join PCI Devices with PCI Device Claims
	// Perform the join using this map[node-addr]=>name
	// This is possible because a (Node, PCIAddress) pair uniquely identifies a PCI Device
	var pdNames map[string]string = make(map[string]string)
	for _, pd := range pds.Items {
		// Build up mapping
		nodeAddr := fmt.Sprintf(
			"%s-%s", pd.Status.NodeName, pd.Status.Address,
		)
		pdNames[nodeAddr] = pd.ObjectMeta.Name

		// Check if PCI Device is already enabled for passthrough, but has no pre-existing PDC,
		// if so, unbind the device (to force the user to make a proper PDC)
		_, found := pdcNames[nodeAddr]
		if !found && pd.Status.KernelDriverInUse == "vfio-pci" && nodename == pd.Status.NodeName {
			logrus.Infof("PCI Device %s is bound to vfio-pci but has no Claim, attempting to unbind", pd.Status.Address)
			err = unbindPCIDeviceFromVfioPCIDriver(pd.Status.Address)
			if err != nil {
				return err
			}
		}
		// After reboot, the PCIDeviceClaim will be there but the PCIDevice won't be bound to vfio-pci
		if pd.Status.KernelDriverInUse != "vfio-pci" && nodename == pd.Status.NodeName {
			// Set PassthroughEnabled to false
			for _, pdc := range pdcs.Items {
				if pdc.Spec.Address == pd.Status.Address {
					logrus.Infof("Passthrough disabled for device %s", pd.Name)
					pdc.Status.PassthroughEnabled = false
					err = unbindPCIDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
					if err != nil {
						logrus.Errorf("Error unbinding device after reboot: %s", err)
					}
					err = addNewIdToVfioPCIDriver(pd.Status.VendorId, pd.Status.DeviceId)
					if err != nil {
						logrus.Errorf("Error rebinding device after reboot: %s", err)
					} else {
						pdc.Status.PassthroughEnabled = true
					}
				}
			}
		}
	}

	// Only load the vfio drivers if there are any PCI Device Claims
	if len(pdcs.Items) > 0 {
		loadVfioDrivers()
	}

	// Get those PCI Device Claims for this node
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName == nodename {
			if pdc.DeletionTimestamp != nil {
				logrus.Infof("Attempting to unbind PCI device %s from vfio-pci", pdc.Spec.Address)
				err = unbindPCIDeviceFromVfioPCIDriver(pdc.Spec.Address)
				if err != nil {
					return err
				}
			}
			if !pdc.Status.PassthroughEnabled {
				logrus.Infof("Attempting to enable passthrough")
				// Get PCIDevice for the PCIDeviceClaim
				name := pdNames[pdc.Spec.NodeAddr()]

				pd, err := h.pdClient.Get(name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				pdc.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
				if pd.Status.KernelDriverInUse == "vfio-pci" {
					pdc.Status.PassthroughEnabled = true
					// Add this device to the KubeVirt CR
					addHostDeviceToKubeVirtAllowList(pd)
				} else {
					// Only unbind from driver is a driver is currently in use
					if strings.TrimSpace(pd.Status.KernelDriverInUse) != "" {
						err = unbindPCIDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
						if err != nil {
							pdc.Status.PassthroughEnabled = false
							return err
						}
					}
					err = enablePassthrough(pd.Status.VendorId, pd.Status.DeviceId, pd.Status.ResourceName)
					if err != nil {
						pdc.Status.PassthroughEnabled = false
						return err
					}
					pdc.Status.PassthroughEnabled = true
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
