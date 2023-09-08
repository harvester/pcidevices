package webhook

import (
	"net/http"

	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/rancher/wrangler/pkg/webhook"
)

func Validation(clients *Clients) (http.Handler, []types.Resource, error) {
	validators := []types.Validator{
		NewSriovNetworkDeviceValidator(clients.PCIFactory.Devices().V1beta1().PCIDeviceClaim().Cache()),
		NewPCIDeviceClaimValidator(clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache(), clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
		NewVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
		NewSRIOVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
	}

	router := webhook.NewRouter()
	resources := make([]types.Resource, 0, len(validators))
	for _, v := range validators {
		addHandler(router, types.AdmissionTypeValidation, types.NewValidatorAdapter(v))
		resources = append(resources, v.Resource())
	}

	return router, resources, nil
}
