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

type PCIDevicesClaimClient func() v1beta1.PCIDeviceClaimInterface

func (p PCIDevicesClaimClient) Update(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p PCIDevicesClaimClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDevicesClaimClient) Create(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (p PCIDevicesClaimClient) Delete(name string, options *metav1.DeleteOptions) error {
	return p().Delete(context.TODO(), name, *options)
}

func (p PCIDevicesClaimClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.PCIDeviceClaimList, error) {
	return p().List(context.TODO(), opts)
}

func (p PCIDevicesClaimClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (p PCIDevicesClaimClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *pcidevicev1beta1.PCIDeviceClaim, err error) {
	panic("implement me")
}

func (p PCIDevicesClaimClient) UpdateStatus(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type PCIDevicesClaimCache func() v1beta1.PCIDeviceClaimInterface

func (p PCIDevicesClaimCache) Get(name string) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDevicesClaimCache) List(selector labels.Selector) ([]*pcidevicev1beta1.PCIDeviceClaim, error) {
	panic("implement me")
}

func (p PCIDevicesClaimCache) AddIndexer(indexName string, indexer pcidevicesv1beta1ctl.PCIDeviceClaimIndexer) {
	panic("implement me")
}

func (p PCIDevicesClaimCache) GetByIndex(indexName, key string) ([]*pcidevicev1beta1.PCIDeviceClaim, error) {
	panic("implement me")
}
