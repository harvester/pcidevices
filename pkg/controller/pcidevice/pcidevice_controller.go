package pcidevice

import (
	"fmt"
	"time"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/iommu"
)

const (
	reconcilePeriod  = time.Second * 20
	pciBridgeClassID = "0604"
)

type Handler struct {
	client                  ctl.PCIDeviceClient
	pci                     *ghw.PCIInfo
	nodeCache               ctlcorev1.NodeCache
	vlanConfigCache         ctlnetworkv1beta1.VlanConfigCache
	sriovNetworkDeviceCache ctl.SRIOVNetworkDeviceCache
	skipAddresses           []string
}

func NewHandler(client ctl.PCIDeviceClient, pci *ghw.PCIInfo, nodeCache ctlcorev1.NodeCache,
	vlanConfigCache ctlnetworkv1beta1.VlanConfigCache, sriovNetworkDeviceCache ctl.SRIOVNetworkDeviceCache, skipAddresses []string) *Handler {
	return &Handler{
		client:                  client,
		pci:                     pci,
		nodeCache:               nodeCache,
		vlanConfigCache:         vlanConfigCache,
		sriovNetworkDeviceCache: sriovNetworkDeviceCache,
		skipAddresses:           skipAddresses,
	}
}

func (h *Handler) ReconcilePCIDevices(nodename string) error {
	// Build up the IOMMU group map
	iommuGroupPaths, err := iommu.GroupPaths()
	if err != nil {
		return err
	}
	iommuGroupMap := iommu.GroupMapForPCIDevices(iommuGroupPaths)

	commonLabels := map[string]string{"nodename": nodename} // label
	var setOfRealPCIAddrs = make(map[string]bool)
	for _, dev := range h.pci.Devices {
		if !containsString(h.skipAddresses, dev.Address) {
			setOfRealPCIAddrs[dev.Address] = true
			name := v1beta1.PCIDeviceNameForHostname(dev.Address, nodename)
			// Check if device is stored
			devCR, err := h.client.Get(name, metav1.GetOptions{})

			if err != nil {
				if apierrors.IsNotFound(err) {
					logrus.Infof("[PCIDeviceController] Device %s does not exist", name)

					// Create the PCIDevice CR if it doesn't exist
					var pdToCreate v1beta1.PCIDevice = v1beta1.NewPCIDeviceForHostname(dev, nodename)
					logrus.Infof("Creating PCI Device: %s\n", err)

					logrus.Debugf("querying sriov network device ownership for pcidevice: %s", pdToCreate.Name)
					commonLabels, err = h.QuerySRIOVNetworkDeviceOwnership(pdToCreate, commonLabels)
					if err != nil {
						return err
					}
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

			// during reboot if the device driver has changed back from vfio, then update the CRD
			// to correct driver in use. This will ensure that the original driver is correctly updated on device
			// the PCIDeviceClaim checks for driver to identify if a rebind is needed on reboot
			if devCopy.Status.KernelDriverInUse != dev.Driver {
				devCopy.Status.KernelDriverInUse = dev.Driver
			}
			// Update only modifies the status, no need to update the main object
			devCopy.Status.Update(dev, nodename, iommuGroupMap) // update the in-memory CR with the current PCI info
			_, err = h.client.UpdateStatus(devCopy)
			if err != nil {
				logrus.Errorf("[PCIDeviceController] Failed to update status sub-resource: %v", err)
				return err
			}
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

func containsString(elements []string, element string) bool {
	for _, v := range elements {
		if v == element {
			return true
		}
	}

	return false
}

// IdentifyPCIBridgeDevices will identify devices which are pci bridges to skip the same
// as these cannot be bound to vfio-pci though share the same iommu group with devices attached
// to the brdige
func IdentifyPCIBridgeDevices(pci *ghw.PCIInfo) []string {
	var pciBridgeAddresses []string
	for _, v := range pci.Devices {
		if fmt.Sprintf("%s%s", v.Class.ID, v.Subclass.ID) == pciBridgeClassID {
			pciBridgeAddresses = append(pciBridgeAddresses, v.Address)
		}
	}
	return pciBridgeAddresses
}

func (h *Handler) QuerySRIOVNetworkDeviceOwnership(device v1beta1.PCIDevice, labels map[string]string) (map[string]string, error) {
	sriovDev, err := h.sriovNetworkDeviceCache.GetByIndex(v1beta1.SRIOVFromVF, device.Name)
	if err != nil {
		return labels, fmt.Errorf("error querying sriov network device: %v", err)
	}

	// expect to find exactly 1 sriovnetworkdevice object in case a pcidevice is VF created by a SriovNetworkDevice
	// no additional labels are added to the pcidevice
	if len(sriovDev) != 1 {
		logrus.Debugf("pcidevice %s is not a VF, no additional labels needed", device.Name)
		return labels, nil
	}
	labels[v1beta1.ParentSRIOVNetworkDevice] = sriovDev[0].Name
	return labels, nil
}
