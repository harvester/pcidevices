package webhook

import (
	"net/http"
	"reflect"

	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/rancher/wrangler/pkg/webhook"
	"github.com/sirupsen/logrus"
)

func Mutation(clients *Clients) (http.Handler, []types.Resource, error) {
	mutators := []types.Mutator{
		NewPodMutator(clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.KubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
			clients.PCIFactory.Devices().V1beta1().VGPUDevice().Cache()),
		NewPCIVMMutator(clients.PCIFactory.Devices().V1beta1().PCIDevice().Cache(),
			clients.PCIFactory.Devices().V1beta1().PCIDeviceClaim().Cache(),
			clients.PCIFactory.Devices().V1beta1().PCIDeviceClaim()),
	}

	router := webhook.NewRouter()
	resources := make([]types.Resource, 0, len(mutators))
	for _, m := range mutators {
		addHandler(router, types.AdmissionTypeMutation, m)
		resources = append(resources, m.Resource())
	}

	return router, resources, nil
}

func addHandler(router *webhook.Router, admissionType string, admitter types.Admitter) {
	rsc := admitter.Resource()
	kind := reflect.Indirect(reflect.ValueOf(rsc.ObjectType)).Type().Name()
	router.Kind(kind).Group(rsc.APIGroup).Type(rsc.ObjectType).Handle(types.NewAdmissionHandler(admitter, admissionType, nil))
	logrus.Infof("add %s handler for %+v.%s (%s)", admissionType, rsc.Names, rsc.APIGroup, kind)
}
