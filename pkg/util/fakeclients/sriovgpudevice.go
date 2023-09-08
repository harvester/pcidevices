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

type SriovGPUDevicesClient func() v1beta1.SRIOVGPUDeviceInterface

func (s SriovGPUDevicesClient) Update(d *pcidevicev1beta1.SRIOVGPUDevice) (*pcidevicev1beta1.SRIOVGPUDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (s SriovGPUDevicesClient) Get(name string, _ metav1.GetOptions) (*pcidevicev1beta1.SRIOVGPUDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s SriovGPUDevicesClient) Create(d *pcidevicev1beta1.SRIOVGPUDevice) (*pcidevicev1beta1.SRIOVGPUDevice, error) {
	return s().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (s SriovGPUDevicesClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s().Delete(context.TODO(), name, *options)
}

func (s SriovGPUDevicesClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.SRIOVGPUDeviceList, error) {
	return s().List(context.TODO(), opts)
}

func (s SriovGPUDevicesClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (s SriovGPUDevicesClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *pcidevicev1beta1.SRIOVGPUDevice, err error) {
	panic("implement me")
}

func (s SriovGPUDevicesClient) UpdateStatus(d *pcidevicev1beta1.SRIOVGPUDevice) (*pcidevicev1beta1.SRIOVGPUDevice, error) {
	return s().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type SriovGPUDevicesCache func() v1beta1.SRIOVGPUDeviceInterface

func (s SriovGPUDevicesCache) Get(name string) (*pcidevicev1beta1.SRIOVGPUDevice, error) {
	return s().Get(context.TODO(), name, metav1.GetOptions{})
}

func (s SriovGPUDevicesCache) List(selector labels.Selector) ([]*pcidevicev1beta1.SRIOVGPUDevice, error) {
	list, err := s().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*pcidevicev1beta1.SRIOVGPUDevice, 0, len(list.Items))
	for _, node := range list.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (s SriovGPUDevicesCache) AddIndexer(_ string, _ pcidevicesv1beta1ctl.SRIOVGPUDeviceIndexer) {
	panic("implement me")
}

func (s SriovGPUDevicesCache) GetByIndex(_, _ string) ([]*pcidevicev1beta1.SRIOVGPUDevice, error) {
	panic("implement me")
}
