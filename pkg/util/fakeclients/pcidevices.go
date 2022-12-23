package fakeclients

import (
	"context"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	pcidevicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type PCIDevicesClient func() v1beta1.PCIDeviceInterface

func (p PCIDevicesClient) Update(d *pcidevicev1beta1.PCIDevice) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p PCIDevicesClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDevicesClient) Create(d *pcidevicev1beta1.PCIDevice) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (p PCIDevicesClient) Delete(name string, options *metav1.DeleteOptions) error {
	return p().Delete(context.TODO(), name, *options)
}

func (p PCIDevicesClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.PCIDeviceList, error) {
	return p().List(context.TODO(), opts)
}

func (p PCIDevicesClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (p PCIDevicesClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *pcidevicev1beta1.PCIDevice, err error) {
	panic("implement me")
}

func (p PCIDevicesClient) UpdateStatus(d *pcidevicev1beta1.PCIDevice) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type PCIDevicesCache func(string) v1beta1.PCIDeviceInterface

func (p PCIDevicesCache) Get(namespace, name string) (*pcidevicev1beta1.PCIDevice, error) {
	return p(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDevicesCache) List(namespace string, selector labels.Selector) ([]*pcidevicev1beta1.PCIDevice, error) {
	panic("implement me")
}

func (p PCIDevicesCache) AddIndexer(indexName string, indexer pcidevicesv1beta1ctl.PCIDeviceClaimIndexer) {
	panic("implement me")
}

func (p PCIDevicesCache) GetByIndex(indexName, key string) ([]**pcidevicev1beta1.PCIDevice, error) {
	panic("implement me")
}
