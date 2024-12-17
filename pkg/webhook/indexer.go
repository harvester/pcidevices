package webhook

import (
	"fmt"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
)

const (
	VMByName                = "harvesterhci.io/vm-by-name"
	PCIDeviceByResourceName = "harvesterhcio.io/pcidevice-by-resource-name"
	IommuGroupByNode        = "pcidevice.harvesterhci.io/iommu-by-node"
	USBDeviceByAddress      = "pcidevice.harvesterhci.io/usb-device-by-address"
	VMByPCIDeviceClaim      = "harvesterhci.io/vm-by-pcideviceclaim"
	VMByUSBDeviceClaim      = "harvesterhci.io/vm-by-usbdeviceclaim"
	VMByVGPU                = "harvesterhci.io/vm-by-vgpu"
	USBDeviceByResourceName = "harvesterhci.io/usbdevice-by-resource-name"
)

func RegisterIndexers(clients *Clients) {
	vmCache := clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmCache.AddIndexer(VMByName, vmByName)
	vmCache.AddIndexer(VMByPCIDeviceClaim, common.VMByHostDeviceName)
	// Because USB device don't have same problem which vGPU and PCI device have,
	// so we just need to use a simple way to collect the host device names.
	vmCache.AddIndexer(VMByUSBDeviceClaim, common.VMBySpecHostDeviceName)
	vmCache.AddIndexer(VMByVGPU, common.VMByVGPUDevice)
	deviceCache := clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache()
	deviceCache.AddIndexer(PCIDeviceByResourceName, pciDeviceByResourceName)
	deviceCache.AddIndexer(IommuGroupByNode, iommuGroupByNodeName)
	usbDeviceCache := clients.DeviceFactory.Devices().V1beta1().USBDevice().Cache()
	usbDeviceCache.AddIndexer(USBDeviceByResourceName, common.USBDeviceByResourceName)
	usbDeviceClaimCache := clients.DeviceFactory.Devices().V1beta1().USBDeviceClaim().Cache()
	usbDeviceClaimCache.AddIndexer(USBDeviceByAddress, usbDeviceClaimByAddress)
}

func vmByName(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	return []string{fmt.Sprintf("%s-%s", obj.Name, obj.Namespace)}, nil
}

func pciDeviceByResourceName(obj *v1beta1.PCIDevice) ([]string, error) {
	return []string{obj.Status.ResourceName}, nil
}

// iommuGroupByNodeName will index the pcidevices by nodename and iommugroup, this will be unique across the cluster
// and can be used to easily query all pcidevices with the same nodename + iommu group combination
func iommuGroupByNodeName(obj *v1beta1.PCIDevice) ([]string, error) {
	return []string{fmt.Sprintf("%s-%s", obj.Status.NodeName, obj.Status.IOMMUGroup)}, nil
}

func usbDeviceClaimByAddress(obj *v1beta1.USBDeviceClaim) ([]string, error) {
	return []string{fmt.Sprintf("%s-%s", obj.Status.NodeName, obj.Status.PCIAddress)}, nil
}
