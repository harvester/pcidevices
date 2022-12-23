package nichelper

import (
	"fmt"

	"github.com/jaypipes/ghw"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var (
	defaultManagedNics = []string{"mgmt-br", "mgmt-bo"}
)

const (
	defaultBRInterface = "mgmt-br"
	defaultBOInterface = "mgmt-bo"
)

func IdentifyManagementNIC() ([]string, error) {
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
