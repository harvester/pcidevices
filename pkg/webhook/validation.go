package webhook

import (
	"net/http"

	"github.com/rancher/wrangler/v3/pkg/webhook"

	"github.com/harvester/harvester/pkg/webhook/types"
)

func Validation(clients *Clients) (http.Handler, []types.Resource, error) {
	validators := []types.Validator{
		NewSriovNetworkDeviceValidator(clients.DeviceFactory.Devices().V1beta1().PCIDeviceClaim().Cache(),
			clients.CoreFactory.Core().V1().Node().Cache()),
		NewPCIDeviceClaimValidator(
			clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.DeviceFactory.Devices().V1beta1().USBDeviceClaim().Cache(),
			clients.DeviceFactory.Devices().V1beta1().USBDevice().Cache(),
			clients.CoreFactory.Core().V1().Node().Cache(),
		),
		NewVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.CoreFactory.Core().V1().Node().Cache()),
		NewSRIOVGPUValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.CoreFactory.Core().V1().Node().Cache()),
		NewUSBDeviceClaimValidator(clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.CoreFactory.Core().V1().Node().Cache()),
		NewDeviceHostValidation(
			clients.DeviceFactory.Devices().V1beta1().USBDevice().Cache(),
			clients.DeviceFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.DeviceFactory.Devices().V1beta1().VGPUDevice().Cache(),
		),
		NewUSBDeviceValidator(clients.CoreFactory.Core().V1().Node().Cache()),
		NewMIGConfigurationValidator(clients.DeviceFactory.Devices().V1beta1().VGPUDevice().Cache()),
	}

	router := webhook.NewRouter()
	resources := make([]types.Resource, 0, len(validators))
	for _, v := range validators {
		addHandler(router, types.AdmissionTypeValidation, types.NewValidatorAdapter(v))
		resources = append(resources, v.Resource())
	}

	return router, resources, nil
}
