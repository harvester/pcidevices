package crd

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	devices "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

func WriteFile(filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return Print(f)
}

func Print(out io.Writer) error {
	obj, err := Objects(false)
	if err != nil {
		return err
	}
	data, err := yaml.Export(obj...)
	if err != nil {
		return err
	}

	objV1Beta1, err := Objects(true)
	if err != nil {
		return err
	}
	dataV1Beta1, err := yaml.Export(objV1Beta1...)
	if err != nil {
		return err
	}

	data = append([]byte("{{- if .Capabilities.APIVersions.Has \"apiextensions.k8s.io/v1\" -}}\n"), data...)
	data = append(data, []byte("{{- else -}}\n---\n")...)
	data = append(data, dataV1Beta1...)
	data = append(data, []byte("{{- end -}}")...)
	_, err = out.Write(data)
	return err
}

func Objects(v1beta1 bool) (result []runtime.Object, err error) {
	for _, crdDef := range List() {
		if v1beta1 {
			crd, err := crdDef.ToCustomResourceDefinitionV1Beta1()
			if err != nil {
				return nil, err
			}
			result = append(result, crd)
		} else {
			crd, err := crdDef.ToCustomResourceDefinition()
			if err != nil {
				return nil, err
			}
			result = append(result, crd)
		}
	}
	return
}

func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&devices.PCIDevice{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Address", ".status.address").
				WithColumn("Vendor Id", ".status.vendorId").
				WithColumn("Device Id", ".status.deviceId").
				WithColumn("Node Name", ".status.nodeName").
				WithColumn("Description", ".status.description").
				WithColumn("Kernel Driver In Use", ".status.kernelDriverInUse")
		}),
		newCRD(&devices.PCIDeviceClaim{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Address", ".spec.address").
				WithColumn("Node Name", ".spec.nodeName").
				WithColumn("User Name", ".spec.userName").
				WithColumn("Kernel Driver Το Unbind", ".status.kernelDriverToUnbind").
				WithColumn("Passthrough Enabled", ".status.passthroughEnabled")
		}),
		newCRD(&devices.SRIOVNetworkDevice{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Address", ".spec.address").
				WithColumn("Node Name", ".spec.nodeName").
				WithColumn("NumVFs", ".spec.numVFs").
				WithColumn("VF Addresses", ".status.vfAddresses")
		}),
		newCRD(&devices.Node{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			c.Status = false
			return c
		}),
		newCRD(&devices.SRIOVGPUDevice{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Address", ".spec.address").
				WithColumn("Node Name", ".spec.nodeName").
				WithColumn("Enabled", ".spec.enabled").
				WithColumn("VGPUDevices", ".status.vGPUDevices")
		}),
		newCRD(&devices.VGPUDevice{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Address", ".spec.address").
				WithColumn("Node Name", ".spec.nodeName").
				WithColumn("Enabled", ".spec.enabled").
				WithColumn("UUID", ".status.uuid").
				WithColumn("VGPUType", ".status.configureVGPUTypeName").
				WithColumn("ParentGPUDevice", ".spec.parentGPUDeviceAddress")
		}),
	}
}

func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, List()...).BatchWait()
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "devices.harvesterhci.io",
			Version: "v1beta1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}
