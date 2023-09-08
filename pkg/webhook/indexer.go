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
	VMByPCIDeviceClaim      = "harvesterhci.io/vm-by-pcideviceclaim"
	VMByVGPU                = "harvesterhci.io/vm-by-vgpu"
)

func RegisterIndexers(clients *Clients) {
	vmCache := clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmCache.AddIndexer(VMByName, vmByName)
	vmCache.AddIndexer(VMByPCIDeviceClaim, vmByPCIDeviceClaim)
	vmCache.AddIndexer(VMByVGPU, vmByVGPUDevice)
	deviceCache := clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache()
	deviceCache.AddIndexer(PCIDeviceByResourceName, pciDeviceByResourceName)
	deviceCache.AddIndexer(IommuGroupByNode, iommuGroupByNodeName)
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
