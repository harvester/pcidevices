package controller

import (
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	sriovctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

func RegisterIndexers(sriovCache sriovctl.SRIOVNetworkDeviceCache) {
	sriovCache.AddIndexer(v1beta1.SRIOVFromVF, getSriovDeviceFromVF)
}

func getSriovDeviceFromVF(obj *v1beta1.SRIOVNetworkDevice) ([]string, error) {
	return obj.Status.VFPCIDevices, nil
}
