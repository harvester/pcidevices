package nichelper

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/net"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/slice"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/harvester/pcidevices/pkg/util/common"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

const (
	defaultBRInterface       = "mgmt-br"
	defaultBOInterface       = "mgmt-bo"
	matchedNodesAnnotation   = "network.harvesterhci.io/matched-nodes"
	defaultHostNetworkNSPath = "/host/proc/1/ns/net"
	defaultTotalVFFile       = "sriov_totalvfs"
	defaultConfiguredVFFile  = "sriov_numvfs"
	interfaceAnnotation      = "sriov.devices.harvesterhi.io/interface-name"
)

var (
	defaultDevicePath = "/sys/bus/pci/devices"
)

func IdentifyHarvesterManagedNIC(nodeName string, nodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache) ([]string, error) {
	var skipInterfaces []string
	managementInterfaces, err := IdentifyManagementNics()
	if err != nil {
		return nil, err
	}

	skipInterfaces = append(skipInterfaces, managementInterfaces...)

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

	// query if NIC is SRIOV capable and already in used with VFs
	sriovInUse, err := ListNICSInUseBySRIOV(nics)
	if err != nil {
		return nil, err
	}

	skipInterfaces = append(skipInterfaces, sriovInUse...)

	for _, v := range skipInterfaces {
		for _, nic := range nics.NICs {
			if nic.Name == v && nic.PCIAddress != nil {
				pciAddresses = append(pciAddresses, *nic.PCIAddress)
			}
		}
	}

	logrus.Debugf("skipping interfaces with pciAddresses: %v", pciAddresses)
	return pciAddresses, nil
}

// IdentifyManagementNics will identify the NICS used on the host for default harvester management
// and bonded interfaces
func IdentifyManagementNics() ([]string, error) {
	hostProcessNS, err := netns.GetFromPath(defaultHostNetworkNSPath)
	if err != nil {
		return nil, fmt.Errorf("error fetching host network namespace: %v", err)
	}
	defer hostProcessNS.Close()

	handler, err := netlink.NewHandleAt(hostProcessNS)
	if err != nil {
		return nil, fmt.Errorf("error generating handler for host network namespace: %v", err)
	}
	defer handler.Close()

	link, err := handler.LinkList()
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
			if l.Attrs().Slave != nil && l.Attrs().MasterIndex == i {
				skipInterfaces = append(skipInterfaces, l.Attrs().Name)
			}
		}
	}

	return skipInterfaces, nil
}

// identifyClusterNetworks will identify vlanConfigs covering the current node and identify NICs in use for
// vlanconfigs
func identifyClusterNetworks(nodeName string, _ ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache) ([]string, error) {
	var nicsList []string
	vlanConfigList, err := vlanConfigCache.List(labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("error fetching vlanconfigs: %v", err)
	}
	for _, v := range vlanConfigList {
		managedNodes, found := v.Annotations[matchedNodesAnnotation]
		if !found { // if annotation not found, ignore as controller keeps checking on regular intervals
			continue
		}
		ok, err := currentNodeMatchesSelector(nodeName, managedNodes)
		if err != nil {
			return nil, fmt.Errorf("error evaluating nodes from selector: %v", err)
		}
		if ok {
			nicsList = append(nicsList, v.Spec.Uplink.NICs...)
		}
	}
	return nicsList, nil
}

// currentNodeMatchesSelector will use the label selectors from VlanConfig to identify if node is
// in the matching the VlanConfig
func currentNodeMatchesSelector(nodeName string, managedNodes string) (bool, error) {
	nodeNames := []string{}
	err := json.Unmarshal([]byte(managedNodes), &nodeNames)
	if err != nil {
		return false, fmt.Errorf("error unmarshalling matched-nodes: %v", err)
	}

	for _, v := range nodeNames {
		if v == nodeName {
			return true, nil
		}
	}
	return false, nil
}

func IsNICSRIOVCapable(deviceAddr string) (bool, error) {
	_, err := os.Stat(filepath.Join(defaultDevicePath, deviceAddr, defaultTotalVFFile))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func CurrentVFConfigured(deviceAddr string) (int, error) {
	devicePath := filepath.Join(defaultDevicePath, deviceAddr)
	return common.CurrentVFConfigured(devicePath)
}

func ConfigureVF(deviceAddr string, numvfs int) error {
	f, err := os.OpenFile(filepath.Join(defaultDevicePath, deviceAddr, defaultConfiguredVFFile), os.O_WRONLY, 0400)
	if err != nil {
		return fmt.Errorf("error opening sriov_numvfs for device %s: %v", deviceAddr, err)
	}

	_, err = f.WriteString(strconv.Itoa(numvfs))
	if err != nil {
		return fmt.Errorf("error writing to sriov_numvfs for device %s: %v", deviceAddr, err)
	}

	defer f.Close()
	return nil
}

func ListNICSInUseBySRIOV(nics *net.Info) ([]string, error) {
	var inUseInterfaces []string
	for _, nic := range nics.NICs {
		if nic.IsVirtual || nic.PCIAddress == nil {
			continue
		}
		ok, err := IsNICSRIOVCapable(*nic.PCIAddress)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		numvfs, err := CurrentVFConfigured(*nic.PCIAddress)
		if err != nil {
			return nil, err
		}

		if numvfs > 0 {
			inUseInterfaces = append(inUseInterfaces, nic.Name)
		}
	}

	return inUseInterfaces, nil
}

// GenerateSRIOVNics will generate list of v1beta1.SRIOVNetworkDevice Objects on regular reconciles to ensure new nics are processed, and nic's used for management
// or cluster network configures are skipped from usage
func GenerateSRIOVNics(nodeName string, nodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache, nics *ghw.NetworkInfo) ([]*v1beta1.SRIOVNetworkDevice, error) {
	var skipNics []string
	clusterNics, err := identifyClusterNetworks(nodeName, nodeCache, vlanConfigCache)
	if err != nil {
		return nil, fmt.Errorf("error listing nics in use for vlanconfig for node %s: %v", nodeName, err)
	}
	skipNics = append(skipNics, clusterNics...)

	mgmtNics, err := IdentifyManagementNics()
	if err != nil {
		return nil, fmt.Errorf("error identifying management nics for node %s: %v", nodeName, err)
	}
	skipNics = append(skipNics, mgmtNics...)

	return generateSRIOVDeviceObjects(nodeName, nics, skipNics)
}

func generateSRIOVDeviceObjects(nodeName string, nics *net.Info, skipNics []string) ([]*v1beta1.SRIOVNetworkDevice, error) {
	var sriovNics []*v1beta1.SRIOVNetworkDevice
	for _, nic := range nics.NICs {
		logrus.Debugf("found device %s on node %s with spec %s", nic.Name, nodeName, nic.String())
		if !slice.ContainsString(skipNics, nic.Name) && !nic.IsVirtual && nic.PCIAddress != nil {
			ok, err := IsNICSRIOVCapable(*nic.PCIAddress)
			if err != nil {
				return nil, fmt.Errorf("error checking if device %s is sriov capable: %v", nic.Name, err)
			}

			if ok {
				sriovDev := generateSRIOVDev(nodeName, nic)
				sriovNics = append(sriovNics, sriovDev)
			}
		}
	}
	return sriovNics, nil
}

func generateSRIOVDev(nodeName string, nic *net.NIC) *v1beta1.SRIOVNetworkDevice {
	return &v1beta1.SRIOVNetworkDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", nodeName, nic.Name),
			Labels: map[string]string{
				v1beta1.NodeKeyName: nodeName,
			},
			Annotations: map[string]string{
				interfaceAnnotation: nic.Name,
			},
		},
		Spec: v1beta1.SRIOVNetworkDeviceSpec{
			Address:  *nic.PCIAddress,
			NodeName: nodeName,
			NumVFs:   0,
		},
	}
}

// OverrideDefaultSysPath is a helper to override the defaultDevicePath variable to
// make it easier to use mocks for testing other packages dependent on nichelper
func OverrideDefaultSysPath(val string) {
	defaultDevicePath = val
}

// GetDefaultSysPath is a helper to fetch the current defaultDevicePath variable
func GetDefaultSysPath() string {
	return defaultDevicePath
}

// copied from: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/v3.0.0/pkg/utils/utils.go#L112
// with minor change to allow override to sysBusPci path to make it easier to test with umockdev
// GetVFList returns a List containing PCI addr for all VF discovered in a given PF
func GetVFList(pf string) (vfList []string, err error) {
	pfDir := filepath.Join(defaultDevicePath, pf)
	return common.GetVFList(pfDir)
}
