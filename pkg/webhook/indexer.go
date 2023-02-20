package webhook

import (
	"fmt"
	"strings"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

const (
	VMByName                = "harvesterhci.io/vm-by-name"
	PCIDeviceByResourceName = "harvesterhcio.io/pcidevice-by-resource-name"
)

func RegisterIndexers(clients *Clients) {
	vmCache := clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()
	vmCache.AddIndexer(VMByName, vmByName)
	deviceCache := clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache()
	deviceCache.AddIndexer(PCIDeviceByResourceName, pciDeviceByResourceName)
}

func pciClaimByVMName(obj *v1beta1.PCIDeviceClaim) ([]string, error) {
	if len(obj.OwnerReferences) != 0 {
		for _, v := range obj.OwnerReferences {
			if strings.ToLower(v.Kind) == "virtualmachine" {
				return []string{v.Name}, nil
			}

		}
	}

	return []string{}, nil
}

func vmByName(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	return []string{fmt.Sprintf("%s-%s", obj.Name, obj.Namespace)}, nil
}

func pciDeviceByResourceName(obj *v1beta1.PCIDevice) ([]string, error) {
	return []string{obj.Status.ResourceName}, nil
}
