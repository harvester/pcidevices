package deviceplugins

import (
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	pluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"
)

func (dp *PCIDevicePlugin) MarkPCIDeviceAsHealthy(resourceName string, pciAddress string) {
	go func() {
		dp.health <- deviceHealth{
			DevId:  pciAddress,
			Health: pluginapi.Healthy,
		}
	}()
}

func (dp *PCIDevicePlugin) MarkPCIDeviceAsUnhealthy(pciAddress string) {
	go func() {
		dp.health <- deviceHealth{
			DevId:  pciAddress,
			Health: pluginapi.Unhealthy,
		}
	}()
}

// Looks for a PCIDevicePlugin with that resourceName, and returns it, or an error if it doesn't exist
func Find(
	resourceName string,
	dps map[string]*PCIDevicePlugin,
) *PCIDevicePlugin {
	dp, found := dps[resourceName]
	if !found {
		return nil
	}
	return dp
}

// Creates a new PCIDevicePlugin with that resourceName, and returns it
func Create(
	resourceName string,
	pciAddressInitial string, // the initial PCI address to mark as healthy
	pdsWithSameResourceName []*v1beta1.PCIDevice,
) *PCIDevicePlugin {
	// Check if there are any PCIDevicePlugins with that resourceName
	pcidevs := []*PCIDevice{}
	for _, pd := range pdsWithSameResourceName {
		pcidevs = append(pcidevs, &PCIDevice{
			pciID:      pd.Status.Address,
			driver:     pd.Status.KernelDriverInUse,
			pciAddress: pd.Status.Address, // this redundancy is here to distinguish between the ID and the PCI Address. They have the same value but mean different things
			iommuGroup: pd.Status.IOMMUGroup,
		})
	}
	// Create the DevicePlugin
	dp := NewPCIDevicePlugin(pcidevs, resourceName)
	dp.MarkPCIDeviceAsHealthy(resourceName, pciAddressInitial)
	return dp
}

// This function adds the PCIDevice to the device plugin, or creates the device plugin if it doesn't exist
func (dp *PCIDevicePlugin) AddDevice(pd *v1beta1.PCIDevice, pdc *v1beta1.PCIDeviceClaim) error {
	_, exists := dp.iommuToPCIMap[pd.Status.Address]

	// made AddDevice idempotent to make reconciles easier
	// if device address doesnt exist in iommuGroupMap then it needs to be added
	// if it exists then no-op
	if !exists {
		resourceName := pd.Status.ResourceName
		logrus.Infof("Adding new claimed %s to device plugin", resourceName)
		pcidevs := []*PCIDevice{{
			pciID:      pd.Status.Address,
			driver:     pd.Status.KernelDriverInUse,
			pciAddress: pd.Status.Address,
			iommuGroup: pd.Status.IOMMUGroup,
		}}
		devs := constructDPIdevices(pcidevs, dp.iommuToPCIMap)
		dp.devs = append(dp.devs, devs...)
		dp.MarkPCIDeviceAsHealthy(resourceName, pdc.Spec.Address)
	}

	return nil
}

// This function adds the PCIDevice to the device plugin, or creates the device plugin if it doesn't exist
func (dp *PCIDevicePlugin) RemoveDevice(pd *v1beta1.PCIDevice, pdc *v1beta1.PCIDeviceClaim) error {
	resourceName := pd.Status.ResourceName
	if dp != nil {
		logrus.Infof("Removing %s from device plugin", resourceName)
		dp.MarkPCIDeviceAsUnhealthy(pdc.Spec.Address)
	}
	return nil
}
