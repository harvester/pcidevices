package controller

import (
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/config"
)

func RegisterIndexers(management *config.FactoryManager) {
	sriovCache := management.DeviceFactory.Devices().V1beta1().SRIOVNetworkDevice().Cache()
	sriovCache.AddIndexer(v1beta1.SRIOVFromVF, getSriovDeviceFromVF)

	usbDevClaimCache := management.DeviceFactory.Devices().V1beta1().USBDeviceClaim().Cache()
	usbDevClaimCache.AddIndexer(v1beta1.USBDevicePCIAddress, getUSBDeviceClaimFromPCIAddress)
}

func getSriovDeviceFromVF(obj *v1beta1.SRIOVNetworkDevice) ([]string, error) {
	return obj.Status.VFPCIDevices, nil
}

func getUSBDeviceClaimFromPCIAddress(obj *v1beta1.USBDeviceClaim) ([]string, error) {
	return []string{obj.Status.PCIAddress}, nil
}
