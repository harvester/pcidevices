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

type PCIDeviceClaimsClient func() v1beta1.PCIDeviceClaimInterface

func (p PCIDeviceClaimsClient) Update(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p PCIDeviceClaimsClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDeviceClaimsClient) Create(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (p PCIDeviceClaimsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return p().Delete(context.TODO(), name, *options)
}

func (p PCIDeviceClaimsClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.PCIDeviceClaimList, error) {
	return p().List(context.TODO(), opts)
}

func (p PCIDeviceClaimsClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (p PCIDeviceClaimsClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *pcidevicev1beta1.PCIDeviceClaim, err error) {
	panic("implement me")
}

func (p PCIDeviceClaimsClient) UpdateStatus(d *pcidevicev1beta1.PCIDeviceClaim) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type PCIDeviceClaimsCache func() v1beta1.PCIDeviceClaimInterface

func (p PCIDeviceClaimsCache) Get(name string) (*pcidevicev1beta1.PCIDeviceClaim, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDeviceClaimsCache) List(selector labels.Selector) ([]*pcidevicev1beta1.PCIDeviceClaim, error) {
	panic("implement me")
}

func (p PCIDeviceClaimsCache) AddIndexer(indexName string, indexer pcidevicesv1beta1ctl.PCIDeviceClaimIndexer) {
	panic("implement me")
}

func (p PCIDeviceClaimsCache) GetByIndex(indexName, key string) ([]*pcidevicev1beta1.PCIDeviceClaim, error) {
	panic("implement me")
}
