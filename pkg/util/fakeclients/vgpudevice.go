package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	pcidevicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

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

func (s VGPUDeviceCache) AddIndexer(_ string, _ pcidevicesv1beta1ctl.VGPUDeviceIndexer) {
	panic("implement me")
}

func (s VGPUDeviceCache) GetByIndex(_, _ string) ([]*pcidevicev1beta1.VGPUDevice, error) {
	panic("implement me")
}
