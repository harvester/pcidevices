package nichelper

import (
	"fmt"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-network-controller/pkg/utils"
	"github.com/jaypipes/ghw"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	defaultManagedNics = []string{"mgmt-br", "mgmt-bo"}
)

const (
	defaultBRInterface = "mgmt-br"
	defaultBOInterface = "mgmt-bo"
)

func IdentifyHarvesterManagedNIC(nodeName string, nodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache) ([]string, error) {
	link, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("error fetching link by name: %v", err)
	}

	// masterBondedIndexes will contain index id for the default bonded interfaces in harvester
	var masterBondedIndexes []int

	logrus.Debug("listing link information")
	for _, l := range link {
		if l.Attrs().Name == defaultBRInterface || l.Attrs().Name == defaultBOInterface {
			masterBondedIndexes = append(masterBondedIndexes, l.Attrs().Index)
		}
	}

	// skipInterfaces contains names of interfaces using in the default harvester bonding interfaces
	// these are to be used by the PCIDevices controller to skip said devices
	var skipInterfaces []string
	for _, i := range masterBondedIndexes {
		for _, l := range link {
			// check helps skip over cases when mgmt-br is also being used for vm networks
			// in which case mgmt-bo is also pointing to mgmt-br
			if l.Attrs().Slave != nil {
				if l.Attrs().MasterIndex == i {
					skipInterfaces = append(skipInterfaces, l.Attrs().Name)
				}
			}
		}
	}

	// query interfaces used for vlanConfigs and add them to list of skipped interfaces
	vlanConfigNICS, err := identifyClusterNetworks(nodeName, nodeCache, vlanConfigCache)
	if err != nil {
		return nil, err
	}

	skipInterfaces = append(skipInterfaces, vlanConfigNICS...)

	logrus.Debugf("skipping interfaces %v", skipInterfaces)

	// pciAddressess contains the pci addresses for the management nics
	var pciAddresses []string
	nics, err := ghw.Network()
	if err != nil {
		return nil, fmt.Errorf("error listing network info: %v", err)
	}

	for _, v := range skipInterfaces {
		for _, nic := range nics.NICs {
			if nic.Name == v {
				pciAddresses = append(pciAddresses, *nic.PCIAddress)
			}
		}
	}

	logrus.Debugf("skipping interfaces with pciAddresses: %v", pciAddresses)
	return pciAddresses, nil
}

// identifyClusterNetworks will identify vlanConfigs covering the current node and identify NICs in use for
// vlanconfigs
func identifyClusterNetworks(nodeName string, nodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache) ([]string, error) {
	var nicsList []string
	vlanConfigList, err := vlanConfigCache.List(labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("error fetching vlanconfigs: %v", err)
	}
	for _, v := range vlanConfigList {
		ok, err := currentNodeMatchesSelector(nodeName, nodeCache, v.Spec.NodeSelector)
		if err != nil {
			return nil, fmt.Errorf("error evaulating nodes from selector: %v", err)
		}
		if ok {
			nicsList = append(nicsList, v.Spec.Uplink.NICs...)
		}
	}
	return nicsList, nil
}

// currentNodeMatchesSelector will use the label selectors from VlanConfig to identify if node is
// in the matching the VlanConfig
func currentNodeMatchesSelector(nodeName string, nodeCache ctlcorev1.NodeCache, vlanConfigLabels map[string]string) (bool, error) {
	selector, err := utils.NewSelector(vlanConfigLabels)
	if err != nil {
		return false, err
	}

	nodes, err := nodeCache.List(selector)
	if err != nil {
		return false, err
	}

	for _, v := range nodes {
		if v.Name == nodeName {
			return true, nil
		}
	}

	return false, nil
}
