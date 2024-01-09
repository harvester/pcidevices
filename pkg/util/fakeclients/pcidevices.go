package fakeclients

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	pcidevicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	IommuGroupByNode = "pcidevice.harvesterhci.io/iommu-by-node"
)

type PCIDevicesClient func() v1beta1.PCIDeviceInterface

func (p PCIDevicesClient) Update(d *pcidevicev1beta1.PCIDevice) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p PCIDevicesClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Get(context.TODO(), name, options)
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

func (p PCIDevicesClient) Watch(metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (p PCIDevicesClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *pcidevicev1beta1.PCIDevice, err error) {
	panic("implement me")
}

func (p PCIDevicesClient) UpdateStatus(d *pcidevicev1beta1.PCIDevice) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type PCIDevicesCache func() v1beta1.PCIDeviceInterface

func (p PCIDevicesCache) Get(name string) (*pcidevicev1beta1.PCIDevice, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PCIDevicesCache) List(labels.Selector) ([]*pcidevicev1beta1.PCIDevice, error) {
	panic("implement me")
}

func (p PCIDevicesCache) AddIndexer(_ string, _ pcidevicesv1beta1ctl.PCIDeviceIndexer) {
	panic("implement me")
}

func (p PCIDevicesCache) GetByIndex(indexName, key string) ([]*pcidevicev1beta1.PCIDevice, error) {
	switch indexName {
	case IommuGroupByNode:
		list, err := p().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		var resp []*pcidevicev1beta1.PCIDevice
		for i, v := range list.Items {
			if key == fmt.Sprintf("%s-%s", v.Status.NodeName, v.Status.IOMMUGroup) {
				resp = append(resp, &list.Items[i])
			}
		}
		return resp, err
	default:
		return nil, nil
	}
}
