package webhook

import (
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"strings"
)

const (
	PCIClaimByVM = "harvesterhci.io/pciclaim-by-vnname"
)

func RegisterIndexers(clients *Clients) {
	pciCache := clients.PCIFactory.Devices().V1beta1().PCIDeviceClaim().Cache()
	pciCache.AddIndexer(PCIClaimByVM, pciClaimByVMName)

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
