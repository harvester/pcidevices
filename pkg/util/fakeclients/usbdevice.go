package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	devicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	devicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type USBDevicesClient func() v1beta1.USBDeviceInterface

func (p USBDevicesClient) Update(d *devicev1beta1.USBDevice) (*devicev1beta1.USBDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p USBDevicesClient) Get(name string, options metav1.GetOptions) (*devicev1beta1.USBDevice, error) {
	return p().Get(context.TODO(), name, options)
}

func (p USBDevicesClient) Create(d *devicev1beta1.USBDevice) (*devicev1beta1.USBDevice, error) {
	return p().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (p USBDevicesClient) Delete(name string, options *metav1.DeleteOptions) error {
	return p().Delete(context.TODO(), name, *options)
}

func (p USBDevicesClient) List(opts metav1.ListOptions) (*devicev1beta1.USBDeviceList, error) {
	return p().List(context.TODO(), opts)
}

func (p USBDevicesClient) Watch(metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (p USBDevicesClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *devicev1beta1.USBDevice, err error) {
	panic("implement me")
}

func (p USBDevicesClient) UpdateStatus(d *devicev1beta1.USBDevice) (*devicev1beta1.USBDevice, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type USBDeviceCache func() v1beta1.USBDeviceInterface

func (p USBDeviceCache) Get(name string) (*devicev1beta1.USBDevice, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p USBDeviceCache) List(selector labels.Selector) ([]*devicev1beta1.USBDevice, error) {
	devices, err := p().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	if err != nil {
		return nil, err
	}

	result := make([]*devicev1beta1.USBDevice, 0, len(devices.Items))

	for _, device := range devices.Items {
		obj := device
		result = append(result, &obj)
	}

	return result, nil
}

func (p USBDeviceCache) AddIndexer(_ string, _ devicesv1beta1ctl.USBDeviceIndexer) {
	panic("implement me")
}

func (p USBDeviceCache) GetByIndex(_, _ string) ([]*devicev1beta1.USBDevice, error) {
	panic("implement me")
}
