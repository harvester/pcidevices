package webhook

import (
	"net/http"

	"github.com/rancher/wrangler/pkg/webhook"

	"github.com/harvester/harvester/pkg/webhook/types"
)

func Validation(clients *Clients) (http.Handler, []types.Resource, error) {
	validators := []types.Validator{
		NewSriovNetworkDeviceValidator(clients.DeviceFactory.Devices().V1beta1().PCIDeviceClaim().Cache()),
		NewPCIDeviceClaimValidator(
			clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.DeviceFactory.Devices().V1beta1().USBDeviceClaim().Cache(),
			clients.DeviceFactory.Devices().V1beta1().USBDevice().Cache(),
		),
		NewVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
		NewSRIOVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
		NewUSBDeviceClaimValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache()),
		NewDeviceHostValidation(
			clients.DeviceFactory.Devices().V1beta1().USBDevice().Cache(),
			clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.DeviceFactory.Devices().V1beta1().VGPUDevice().Cache(),
		),
		NewUSBDeviceValidator(),
	}

	router := webhook.NewRouter()
	resources := make([]types.Resource, 0, len(validators))
	for _, v := range validators {
		addHandler(router, types.AdmissionTypeValidation, types.NewValidatorAdapter(v))
		resources = append(resources, v.Resource())
	}

	return router, resources, nil
}
