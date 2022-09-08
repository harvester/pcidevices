package pcideviceclaim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	pdcClient v1beta1gen.PCIDeviceClaimClient,
	pd v1beta1gen.PCIDeviceClient,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	handler := &Handler{
		pdcClient: pdcClient,
		pdClient:  pd,
	}
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	pdcClient.OnRemove(ctx, "", handler.OnRemove)
	// start goroutine to regularly reconcile the PCI Device Claims' status with their spec
	go func() {
		ticker := time.NewTicker(reconcilePeriod)
		for range ticker.C {
			logrus.Info("Reconciling PCI Device Claims list")
			if err := handler.reconcilePCIDeviceClaims(hostname); err != nil {
				logrus.Errorf("PCI Device Claim reconciliation error: %v", err)
			}
		}
	}()
	return nil
}

func (c *Controller) OnChange(key string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	logrus.Infof("PCI Device Claim %s has changed", pdc.Name)
	return pdc, nil
}

func vfioPCIDriverIsLoaded() bool {
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "vfio_pci") {
			return true
		}
	}
	return false
}

func loadVfioPCIDriver() {
	cmd := exec.Command("modprobe", "vfio-pci")
	err := cmd.Run()
	if err != nil {
		logrus.Error(err)
	}
}

func addNewIdToVfioPCIDriver(vendorId int, deviceId int) error {
	var id string = fmt.Sprintf("%x %x", vendorId, deviceId)

	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/new_id", os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = file.WriteString(id)
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

		// Check if PCI Device is already enabled for passthrough,
		// but has no pre-existing PDC, if so, create a PDC
		_, found := pdcNames[nodeAddr]
		if !found && pd.Status.KernelDriverInUse == "vfio-pci" && hostname == pd.Status.NodeName {
			addrDNSsafe := strings.ReplaceAll(strings.ReplaceAll(pd.Status.Address, ":", ""), ".", "")
			pdcName := fmt.Sprintf(
				"%s-%s-%x-%x-%s",
				hostname,
				"vendor", // TODO vendorName
				pd.Status.VendorId,
				pd.Status.DeviceId,
				addrDNSsafe,
			)
			pdc := v1beta1.PCIDeviceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: pdcName,
				},
				Spec: v1beta1.PCIDeviceClaimSpec{
					Address:  pd.Status.Address,
					NodeName: pd.Status.NodeName,
					UserName: "admin", // if there's no pdc but the device is claimed, assume admin user
				},
			}
			logrus.Infof("PCI Device %s is bound to vfio-pci but has no Claim, attempting to create", pd.Status.Address)
			_, err := h.pdcClient.Create(&pdc)
			if err != nil {
				return err
			}
		}
	}

	// Load the PCI Passthrough VFIO driver if necessary
	if !vfioPCIDriverIsLoaded() {
		loadVfioPCIDriver()
	}

	// Get those PCI Device Claims for this node
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName == hostname {
			if !pdc.Status.PassthroughEnabled {
				logrus.Infof("Attempting to enable passthrough")
				// Get PCIDevice for the PCIDeviceClaim
				name := pdNames[pdc.Spec.NodeAddr()]
				pd, err := h.pdClient.Get(name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				pdc.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
				// TODO Check if kernelDriver is non-empty
				// Add device in PCIDeviceClaim to the vfio-pci driver's list of devices
				if pd.Status.KernelDriverInUse != "vfio-pci" {
					err = addNewIdToVfioPCIDriver(pd.Status.VendorId, pd.Status.DeviceId)
					if err != nil {
						pdc.Status.PassthroughEnabled = false
						return err
					}
					pdc.Status.PassthroughEnabled = true
				}
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

func (c *Controller) OnRemove(
	ctx context.Context,
	name string,
	sync v1beta1gen.PCIDeviceClaimHandler,
) {
	logrus.Infof("PCIDeviceClaim %s removed, unbinding the device from vfio-pci driver", name)
	// Unbind the device
	pdc, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get PCIDeviceClaim %s: %s", name, err)
		return
	}
	var addr string = pdc.Spec.Address
	err = unbindPCIDeviceFromVfioPCIDriver(addr)
	if err != nil {
		logrus.Errorf("Failed to unbind PCIDevice from vfio-driver: %s", err)
	}
}
