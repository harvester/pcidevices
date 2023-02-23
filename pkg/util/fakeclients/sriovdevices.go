package fakeclients

import (
	"context"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	pcidevicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/rancher/wrangler/pkg/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type SriovDevicesClient func() v1beta1.SRIOVNetworkDeviceInterface

func (s SriovDevicesClient) Update(d *pcidevicev1beta1.SRIOVNetworkDevice) (*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (s SriovDevicesClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s SriovDevicesClient) Create(d *pcidevicev1beta1.SRIOVNetworkDevice) (*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	return s().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (s SriovDevicesClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s().Delete(context.TODO(), name, *options)
}

func (s SriovDevicesClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.SRIOVNetworkDeviceList, error) {
	return s().List(context.TODO(), opts)
}

func (s SriovDevicesClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (s SriovDevicesClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *pcidevicev1beta1.SRIOVNetworkDevice, err error) {
	panic("implement me")
}

func (s SriovDevicesClient) UpdateStatus(d *pcidevicev1beta1.SRIOVNetworkDevice) (*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type SriovDevicesCache func() v1beta1.SRIOVNetworkDeviceInterface

func (s SriovDevicesCache) Get(name string) (*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s SriovDevicesCache) List(selector labels.Selector) ([]*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	list, err := s().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*pcidevicev1beta1.SRIOVNetworkDevice, 0, len(list.Items))
	for _, node := range list.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (s SriovDevicesCache) AddIndexer(indexName string, indexer pcidevicesv1beta1ctl.SRIOVNetworkDeviceIndexer) {
	panic("implement me")
}

func (s SriovDevicesCache) GetByIndex(indexName, key string) ([]*pcidevicev1beta1.SRIOVNetworkDevice, error) {
	switch indexName {
	case pcidevicev1beta1.SRIOVFromVF:
		sriovDevList, err := s.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}

		var sriovNetworkDevices []*pcidevicev1beta1.SRIOVNetworkDevice
		for _, v := range sriovDevList {
			if slice.ContainsString(v.Status.VFPCIDevices, key) {
				sriovNetworkDevices = append(sriovNetworkDevices, v)
			}
		}

		return sriovNetworkDevices, nil
	default:
		panic("implement me")
	}
}
