package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	devicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
)

const USBDeviceByResourceName = "harvesterhci.io/usbdevice-by-resource-name"

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

func (p USBDevicesClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*devicev1beta1.USBDevice, *devicev1beta1.USBDeviceList], error) {
	panic("implement me")
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

func (p USBDeviceCache) AddIndexer(_ string, _ generic.Indexer[*devicev1beta1.USBDevice]) {
	panic("implement me")
}

func (p USBDeviceCache) GetByIndex(indexName, name string) ([]*devicev1beta1.USBDevice, error) {
	switch indexName {
	case USBDeviceByResourceName:
		var usbDevices []*devicev1beta1.USBDevice
		devices, err := p.List(labels.NewSelector())
		if err != nil {
			return []*devicev1beta1.USBDevice{}, err
		}

		for _, device := range devices {
			if device.Status.ResourceName == name {
				usbDevices = append(usbDevices, device)
			}
		}
		return usbDevices, err
	default:
	}

	return []*devicev1beta1.USBDevice{}, nil
}
