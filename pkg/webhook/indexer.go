package webhook

import (
	"fmt"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
)

const (
	VMByName                 = "harvesterhci.io/vm-by-name"
	PCIDeviceByResourceName  = "harvesterhcio.io/pcidevice-by-resource-name"
	IommuGroupByNode         = "pcidevice.harvesterhci.io/iommu-by-node"
	USBDeviceByAddress       = "pcidevice.harvesterhci.io/usb-device-by-address"
	VMByPCIDeviceClaim       = "harvesterhci.io/vm-by-pcideviceclaim"
	VMByUSBDeviceClaim       = "harvesterhci.io/vm-by-usbdeviceclaim"
	VMByVGPU                 = "harvesterhci.io/vm-by-vgpu"
	USBDeviceByResourceName  = "harvesterhci.io/usbdevice-by-resource-name"
	vGPUDeviceByResourceName = "harvesterhci.io/vgpu-device-by-resource-name"
)

func RegisterIndexers(clients *Clients) {
	vmCache := clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmCache.AddIndexer(VMByName, vmByName)
	vmCache.AddIndexer(VMByPCIDeviceClaim, vmByPCIDeviceClaim)
	vmCache.AddIndexer(VMByVGPU, vmByVGPUDevice)
	deviceCache := clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache()
	deviceCache.AddIndexer(PCIDeviceByResourceName, pciDeviceByResourceName)
	deviceCache.AddIndexer(IommuGroupByNode, iommuGroupByNodeName)

	vgpuCache := clients.DeviceFactory.Devices().V1beta1().VGPUDevice().Cache()
	vgpuCache.AddIndexer(vGPUDeviceByResourceName, common.VGPUDeviceByResourceName)
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

// vmByPCIDeviceClaim indexes VM's by device claim names.
func vmByPCIDeviceClaim(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	pciDeviceClaimNames := make([]string, 0, len(obj.Spec.Template.Spec.Domain.Devices.HostDevices))
	for _, hostDevice := range obj.Spec.Template.Spec.Domain.Devices.HostDevices {
		pciDeviceClaimNames = append(pciDeviceClaimNames, hostDevice.Name)
	}
	return pciDeviceClaimNames, nil
}

// vmByVGPUDevice indexes VM's by vgpu names
func vmByVGPUDevice(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	gpuNames := make([]string, 0, len(obj.Spec.Template.Spec.Domain.Devices.GPUs))
	for _, gpuDevice := range obj.Spec.Template.Spec.Domain.Devices.GPUs {
		gpuNames = append(gpuNames, gpuDevice.Name)
	}
	return gpuNames, nil
}
