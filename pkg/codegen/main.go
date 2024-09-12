package main

import (
	"fmt"
	"os"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/crd"
)

func main() {
	if len(os.Args) > 2 && os.Args[1] == "crds" {
		fmt.Println("Writing CRDs to", os.Args[2])
		if err := crd.WriteFile(os.Args[2]); err != nil {
			panic(err)
		}
		return
	}

	os.Unsetenv("GOPATH")
	controllergen.Run(
		args.Options{
			OutputPackage: "github.com/harvester/pcidevices/pkg/generated",
			Boilerplate:   "scripts/boilerplate.go.txt",
			Groups: map[string]args.Group{
				"devices.harvesterhci.io": {
					Types: []any{
						"./pkg/apis/devices.harvesterhci.io/v1beta1",
					},
					GenerateTypes:   true,
					GenerateClients: true,
				},
				kubevirtv1.SchemeGroupVersion.Group: {
					Types: []interface{}{
						kubevirtv1.KubeVirt{},
						kubevirtv1.VirtualMachineInstance{},
						kubevirtv1.VirtualMachine{},
					},
					GenerateTypes:   false,
					GenerateClients: true,
				},
			},
		})
}
