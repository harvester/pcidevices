package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"github.com/rancher/wrangler/v3/pkg/generic"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
)

const vGPUDeviceByResourceName = "harvesterhci.io/vgpu-device-by-resource-name"

type VGPUDeviceClient func() v1beta1.VGPUDeviceInterface

func (s VGPUDeviceClient) Update(d *pcidevicev1beta1.VGPUDevice) (*pcidevicev1beta1.VGPUDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (s VGPUDeviceClient) Get(name string, _ metav1.GetOptions) (*pcidevicev1beta1.VGPUDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s VGPUDeviceClient) Create(d *pcidevicev1beta1.VGPUDevice) (*pcidevicev1beta1.VGPUDevice, error) {
	return s().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (s VGPUDeviceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s().Delete(context.TODO(), name, *options)
}

func (s VGPUDeviceClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.VGPUDeviceList, error) {
	return s().List(context.TODO(), opts)
}

func (s VGPUDeviceClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (s VGPUDeviceClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *pcidevicev1beta1.VGPUDevice, err error) {
	panic("implement me")
}

func (s VGPUDeviceClient) UpdateStatus(d *pcidevicev1beta1.VGPUDevice) (*pcidevicev1beta1.VGPUDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (s VGPUDeviceClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*pcidevicev1beta1.VGPUDevice, *pcidevicev1beta1.VGPUDeviceList], error) {
	panic("implement me")
}

type VGPUDeviceCache func() v1beta1.VGPUDeviceInterface

func (s VGPUDeviceCache) Get(name string) (*pcidevicev1beta1.VGPUDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s VGPUDeviceCache) List(selector labels.Selector) ([]*pcidevicev1beta1.VGPUDevice, error) {
	list, err := s().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*pcidevicev1beta1.VGPUDevice, 0, len(list.Items))
	for _, node := range list.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (s VGPUDeviceCache) AddIndexer(_ string, _ generic.Indexer[*pcidevicev1beta1.VGPUDevice]) {
	panic("implement me")
}

func (s VGPUDeviceCache) GetByIndex(index, key string) ([]*pcidevicev1beta1.VGPUDevice, error) {
	switch index {
	case vGPUDeviceByResourceName:
		devices, err := s.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
		for _, device := range devices {
			if common.GeneratevGPUDeviceName(device.Status.ConfiguredVGPUTypeName) == key {
				return []*pcidevicev1beta1.VGPUDevice{device}, nil
			}
		}
		return nil, nil
	default:
	}

	return nil, nil
}
