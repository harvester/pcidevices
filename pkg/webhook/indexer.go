package webhook

import (
	"fmt"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

const (
	VMByName                = "harvesterhci.io/vm-by-name"
	PCIDeviceByResourceName = "harvesterhcio.io/pcidevice-by-resource-name"
	IommuGroupByNode        = "pcidevice.harvesterhci.io/iommu-by-node"
	USBDeviceByAddress      = "pcidevice.harvesterhci.io/usb-device-by-address"
	VMByPCIDeviceClaim      = "harvesterhci.io/vm-by-pcideviceclaim"
	VMByUSBDeviceClaim      = "harvesterhci.io/vm-by-usbdeviceclaim"
	VMByVGPU                = "harvesterhci.io/vm-by-vgpu"
)

func RegisterIndexers(clients *Clients) {
	vmCache := clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmCache.AddIndexer(VMByName, vmByName)
	vmCache.AddIndexer(VMByPCIDeviceClaim, vmByHostDeviceName)
	vmCache.AddIndexer(VMByUSBDeviceClaim, vmByHostDeviceName)
	vmCache.AddIndexer(VMByVGPU, vmByVGPUDevice)
	deviceCache := clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache()
	deviceCache.AddIndexer(PCIDeviceByResourceName, pciDeviceByResourceName)
	deviceCache.AddIndexer(IommuGroupByNode, iommuGroupByNodeName)
	usbDeviceClaimCache := clients.PCIFactory.Devices().V1beta1().USBDeviceClaim().Cache()
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

// vmByHostDeviceName indexes VM's by host device name.
// It could be usb device claim or pci device claim name.
func vmByHostDeviceName(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	hostDeviceName := make([]string, 0, len(obj.Spec.Template.Spec.Domain.Devices.HostDevices))
	for _, hostDevice := range obj.Spec.Template.Spec.Domain.Devices.HostDevices {
		hostDeviceName = append(hostDeviceName, hostDevice.Name)
	}
	return hostDeviceName, nil
}

// vmByVGPUDevice indexes VM's by vgpu names
func vmByVGPUDevice(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	gpuNames := make([]string, 0, len(obj.Spec.Template.Spec.Domain.Devices.GPUs))
	for _, gpuDevice := range obj.Spec.Template.Spec.Domain.Devices.GPUs {
		gpuNames = append(gpuNames, gpuDevice.Name)
	}
	return gpuNames, nil
}

func usbDeviceClaimByAddress(obj *v1beta1.USBDeviceClaim) ([]string, error) {
	return []string{fmt.Sprintf("%s-%s", obj.Status.NodeName, obj.Status.PCIAddress)}, nil
}
