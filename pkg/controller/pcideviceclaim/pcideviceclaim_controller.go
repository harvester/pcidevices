package pcideviceclaim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/kmodule"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	reconcilePeriod = time.Minute * 1
	vfioPCIDriver   = "vfio-pci"
)

type Controller struct {
	PCIDeviceClaims v1beta1gen.PCIDeviceClaimController
}

type Handler struct {
	pdcClient  v1beta1gen.PCIDeviceClaimClient
	pdClient   v1beta1gen.PCIDeviceClient
	virtClient kubecli.KubevirtClient
}

func Register(
	ctx context.Context,
	pdcClient v1beta1gen.PCIDeviceClaimController,
	pdClient v1beta1gen.PCIDeviceController,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt client: %v", err)
		return errors.New(msg)
	}
	handler := &Handler{
		pdcClient:  pdcClient,
		pdClient:   pdClient,
		virtClient: virtClient,
	}
	pdcClient.OnRemove(ctx, "PCIDeviceClaimOnRemove", handler.OnRemove)
	pdcClient.OnChange(ctx, "PCIDeviceClaimOnChange", handler.OnChange)
	nodename := os.Getenv("NODE_NAME")
	handler.rebindAfterReboot(nodename)
	err = handler.unbindOrphanedPCIDevices(nodename)
	if err != nil {
		return err
	}
	// Load VFIO drivers when controller starts instead of repeatedly in the reconcile loop
	loadVfioDrivers()

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

func (h Handler) OnChange(name string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	if pdc != nil && pdc.Spec.NodeName != os.Getenv("NODE_NAME") {
		return pdc, nil
	}
	if pdc == nil {
		return nil, nil
	}
	_, err := h.pdcClient.Get(name, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("OnCreate(%s, %s)", name, pdc.Name)
			pdc, err = h.pdcClient.Create(pdc)
			if err != nil {
				return pdc, err
			}
			err = h.attemptToEnablePassthrough(pdc)
			if err != nil {
				return pdc, err
			}
			return pdc, nil
		}
		return pdc, err
	}

	return pdc, nil
}

// When a PCIDeviceClaim is removed, we need to unbind the device from the vfio-pci driver
func (h Handler) OnRemove(name string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	if pdc.Spec.NodeName != os.Getenv("NODE_NAME") {
		return pdc, nil
	}
	if pdc == nil {
		return nil, nil
	}
	logrus.Infof("OnRemove(%s, %s)", name, pdc.Name)
	if pdc == nil {
		return nil, nil
	}
	err := h.attemptToDisablePassthrough(pdc)
	if err != nil {
		return pdc, err
	}

	return pdc, nil
}

func loadVfioDrivers() {
	for _, driver := range []string{"vfio-pci", "vfio_iommu_type1"} {
		logrus.Infof("Loading driver %s", driver)
		if err := kmodule.Probe(driver, ""); err != nil {
			logrus.Error(err)
		}
	}
}

func bindDeviceToVFIOPCIDriver(pd *v1beta1.PCIDevice) error {
	vendorId := pd.Status.VendorId
	deviceId := pd.Status.DeviceId
	var id string = fmt.Sprintf("%s %s", vendorId, deviceId)
	logrus.Infof("Binding device %s [%s] to vfio-pci", pd.Name, id)

	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/new_id", os.O_WRONLY, 0400)
	if err != nil {
		logrus.Errorf("Error opening new_id file: %s", err)
		return err
	}
	_, err = file.WriteString(id)
	if err != nil {
		logrus.Errorf("Error writing to new_id file: %s", err)
		file.Close()
		return err
	}
	file.Close()
	return nil
}

// Enabling passthrough for a PCI Device requires two steps:
// 1. Bind the device to the vfio-pci driver in the host
// 2. Permit the device in KubeVirt's Config in the cluster
func (h Handler) enablePassthrough(pd *v1beta1.PCIDevice) error {
	err := bindDeviceToVFIOPCIDriver(pd)
	if err != nil {
		return err
	}
	err = h.addHostDeviceToKubeVirt(pd)
	if err != nil {
		return err
	}
	return nil
}

func (h Handler) disablePassthrough(pd *v1beta1.PCIDevice) error {
	errDriver := unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
	errKubeVirt := h.removeHostDeviceFromKubeVirt(pd)
	if errDriver != nil && errKubeVirt == nil {
		return errDriver
	}
	if errDriver == nil && errKubeVirt != nil {
		return errKubeVirt
	}
	if errDriver != nil && errKubeVirt != nil {
		msg := fmt.Sprintf("failed unbinding driver and also kubevirt: (%s, %s)", errDriver, errKubeVirt)
		return errors.New(msg)
	}
	return nil
}

// This function unbinds the device with PCI Address addr from the given driver
// NOTE: this function assumes that addr is on THIS NODE, only call for PCI addrs on this node
func unbindDeviceFromDriver(addr string, driver string) error {
	driverPath := fmt.Sprintf("/sys/bus/pci/drivers/%s", driver)
	// Check if device at addr is already bound to driver
	_, err := os.Stat(fmt.Sprintf("%s/%s", driverPath, addr))
	if err != nil {
		logrus.Errorf("Device at address %s is not bound to driver %s", addr, driver)
		return nil
	}
	path := fmt.Sprintf("%s/unbind", driverPath)
	file, err := os.OpenFile(path, os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(addr)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func (h Handler) removeHostDeviceFromKubeVirt(pd *v1beta1.PCIDevice) error {
	logrus.Infof("Removing %s from KubeVirt list of permitted devices", pd.Name)
	ns := "harvester-system"
	cr := "kubevirt"
	kv, err := h.virtClient.KubeVirt(ns).Get(cr, &v1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt CR: %v", err)
		return errors.New(msg)
	}
	kvCopy := kv.DeepCopy()
	var indexToRemove int = -1
	for i, pciHostDevice := range kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices {
		if pciHostDevice.ResourceName == pd.Status.ResourceName {
			// Remove from list of devices
			indexToRemove = i
			break
		}
	}
	if indexToRemove == -1 {
		msg := fmt.Sprintf("PCIDevice %s is not in the list of permitted devices", pd.Name)
		logrus.Infof(msg)
		return nil // returning err would cause a requeuing, instead we log error and return nil
	}
	// To delete the element, just move the last one to the indexToRemove and shrink the slice by 1
	s := kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices
	s[indexToRemove] = s[len(s)-1]
	s = s[:len(s)-1]
	kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices = s
	h.virtClient.KubeVirt(ns).Update(kvCopy)
	_, err = h.virtClient.KubeVirt(ns).Update(kvCopy)
	if err != nil {
		msg := fmt.Sprintf("Failed to update kubevirt CR: %s", err)
		return errors.New(msg)
	}
	return nil
}

func (h Handler) addHostDeviceToKubeVirt(pd *v1beta1.PCIDevice) error {
	logrus.Infof("Adding %s to KubeVirt list of permitted devices", pd.Name)
	ns := "harvester-system"
	cr := "kubevirt"
	kv, err := h.virtClient.KubeVirt(ns).Get(cr, &v1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt CR: %v", err)
		return errors.New(msg)
	}
	kvCopy := kv.DeepCopy()
	if kv.Spec.Configuration.PermittedHostDevices == nil {
		kvCopy.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			PciHostDevices: []kubevirtv1.PciHostDevice{},
		}
	}
	permittedPCIDevices := kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices
	resourceName := pd.Status.ResourceName
	// check if device is currently permitted
	var devPermitted bool = false
	for _, permittedPCIDev := range permittedPCIDevices {
		if permittedPCIDev.ResourceName == resourceName {
			devPermitted = true
			break
		}
	}
	if !devPermitted {
		vendorId := pd.Status.VendorId
		deviceId := pd.Status.DeviceId
		devToPermit := kubevirtv1.PciHostDevice{
			PCIVendorSelector:        fmt.Sprintf("%s:%s", vendorId, deviceId),
			ResourceName:             resourceName,
			ExternalResourceProvider: false,
		}
		kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices = append(permittedPCIDevices, devToPermit)
		_, err := h.virtClient.KubeVirt(ns).Update(kvCopy)
		if err != nil {
			msg := fmt.Sprintf("Failed to update kubevirt CR: %s", err)
			return errors.New(msg)
		}
	}
	return nil
}

func pciDeviceIsClaimed(pd *v1beta1.PCIDevice, pdcs *v1beta1.PCIDeviceClaimList) bool {
	for _, pdc := range pdcs.Items {
		if pdc.OwnerReferences == nil {
			return false
		}
		if pdc.OwnerReferences[0].Name == pd.Name {
			return true
		}
	}
	return false
}

// A PCI Device is considered orphaned if it is bound to vfio-pci,
// but has no PCIDeviceClaim. The assumption is that this controller
// will manage all PCI passthrough, and consider orphaned devices invalid
func getOrphanedPCIDevices(
	nodename string,
	pdcs *v1beta1.PCIDeviceClaimList,
	pds *v1beta1.PCIDeviceList,
) (*v1beta1.PCIDeviceList, error) {
	pdsOrphaned := v1beta1.PCIDeviceList{}
	for _, pd := range pds.Items {
		isVfioPci := pd.Status.KernelDriverInUse == "vfio-pci"
		isOnThisNode := nodename == pd.Status.NodeName
		if isVfioPci && isOnThisNode && !pciDeviceIsClaimed(&pd, pdcs) {
			pdsOrphaned.Items = append(pdsOrphaned.Items, *pd.DeepCopy())
		}
	}
	return &pdsOrphaned, nil
}

// After reboot, the PCIDeviceClaim will be there but the PCIDevice won't be bound to vfio-pci
func (h Handler) rebindAfterReboot(nodename string) {
	logrus.Infof("Rebinding after reboot on node: %s", nodename)
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error getting claims: %s", err)
		return
	}
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName != nodename {
			continue
		}
		// Get PCIDevice for the PCIDeviceClaim
		name := pdc.OwnerReferences[0].Name
		pd, err := h.pdClient.Get(name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Error getting claim's device: %s", err)
			continue
		}

		if pd.Status.KernelDriverInUse == "vfio-pci" {
			logrus.Infof("PCIDevice %s is already bound to vfio-pci, skipping", pd.Name)
			continue
		}

		logrus.Infof("Passthrough disabled for device %s", pd.Name)
		pdcCopy := pdc.DeepCopy()

		// Try to unbind from existing driver, if applicable
		err = unbindDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
		if err != nil {
			pdcCopy.Status.PassthroughEnabled = true
			logrus.Errorf("Error unbinding device after reboot: %s", err)
		} else {
			pdcCopy.Status.PassthroughEnabled = false
		}

		// Enable Passthrough on the device
		err = h.enablePassthrough(pd)
		if err != nil {
			logrus.Errorf("Error rebinding device after reboot: %s", err)
			pdcCopy.Status.PassthroughEnabled = false
		} else {
			pdcCopy.Status.PassthroughEnabled = true
		}
		_, err = h.pdcClient.UpdateStatus(pdcCopy)
		if err != nil {
			logrus.Errorf("Failed to update PCIDeviceClaim status for %s: %s", pdc.Name, err)
		}
	}
}

func (h Handler) reconcilePCIDeviceClaims(nodename string) error {
	// Get all PCI Device Claims on this node
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pdc := range pdcs.Items {
		// TODO change UI to label each PDC to allow for the LabelSelector instead of testing all PDCs
		if pdc.Spec.NodeName != nodename {
			continue
		}
		if !pdc.Status.PassthroughEnabled {
			err := h.attemptToEnablePassthrough(&pdc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h Handler) attemptToEnablePassthrough(pdc *v1beta1.PCIDeviceClaim) error {
	logrus.Infof("Attempting to enable passthrough for %s", pdc.Name)
	// Get PCIDevice for the PCIDeviceClaim
	if pdc.OwnerReferences == nil {
		msg := fmt.Sprintf("Cannot find PCIDevice that owns %s", pdc.Name)
		return errors.New(msg)
	}
	name := pdc.OwnerReferences[0].Name
	pd, err := h.pdClient.Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error getting claim's device: %s", err)
		return err
	}
	pdcCopy := pdc.DeepCopy()
	pdcCopy.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
	if pd.Status.KernelDriverInUse == vfioPCIDriver {
		pdcCopy.Status.PassthroughEnabled = true
		err = h.addHostDeviceToKubeVirt(pd)
		if err != nil {
			return err
		}
	} else {
		// Only unbind from driver is a driver is currently in use
		if strings.TrimSpace(pd.Status.KernelDriverInUse) != "" {
			err = unbindDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
			if err != nil {
				pdcCopy.Status.PassthroughEnabled = false
			}
		}
		// Enable PCI Passthrough by binding the device to the vfio-pci driver
		err = h.enablePassthrough(pd)
		if err != nil {
			pdcCopy.Status.PassthroughEnabled = false
		} else {
			pdcCopy.Status.PassthroughEnabled = true
		}
	}
	_, err = h.pdcClient.UpdateStatus(pdcCopy)
	if err != nil {
		logrus.Errorf("Error updating status for %s: %s", pdc.Name, err)
		return err
	}
	return nil
}

func (h Handler) attemptToDisablePassthrough(pdc *v1beta1.PCIDeviceClaim) error {
	logrus.Infof("Attempting to disable passthrough for %s", pdc.Name)
	// Get PCIDevice for the PCIDeviceClaim
	name := pdc.OwnerReferences[0].Name
	pd, err := h.pdClient.Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error getting claim's device: %s", err)
		return err
	}
	pdcCopy := pdc.DeepCopy()
	pdcCopy.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
	if pd.Status.KernelDriverInUse == vfioPCIDriver {
		pdcCopy.Status.PassthroughEnabled = true
		err = h.removeHostDeviceFromKubeVirt(pd)
		if err != nil {
			return err
		}
		// Only unbind from driver is a driver is currently bound to vfio
		if strings.TrimSpace(pd.Status.KernelDriverInUse) == vfioPCIDriver {
			err = unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
			if err != nil {
				pdcCopy.Status.PassthroughEnabled = true
			}
		}
		// Enable PCI Passthrough by binding the device to the vfio-pci driver
		err = h.disablePassthrough(pd)
		if err != nil {
			pdcCopy.Status.PassthroughEnabled = true
		} else {
			pdcCopy.Status.PassthroughEnabled = false
		}
	}
	_, err = h.pdcClient.UpdateStatus(pdcCopy)
	if err != nil {
		logrus.Errorf("Error updating status for %s: %s", pdc.Name, err)
		return err
	}
	return nil
}

func (h Handler) unbindOrphanedPCIDevices(nodename string) error {
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pds, err := h.pdClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	orphanedPCIDevices, err := getOrphanedPCIDevices(nodename, pdcs, pds)
	if err != nil {
		return err
	}
	for _, pd := range orphanedPCIDevices.Items {
		unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
	}
	return nil
}
