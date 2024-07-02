package fakeclients

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	devicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
	devicesv1beta1ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const USBDeviceByAddress = "pcidevice.harvesterhci.io/usb-device-by-address"

type USBDeviceClaimsClient func() v1beta1.USBDeviceClaimInterface

func (p USBDeviceClaimsClient) Update(d *devicev1beta1.USBDeviceClaim) (*devicev1beta1.USBDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (p USBDeviceClaimsClient) Get(name string, options metav1.GetOptions) (*devicev1beta1.USBDeviceClaim, error) {
	return p().Get(context.TODO(), name, options)
}

func (p USBDeviceClaimsClient) Create(d *devicev1beta1.USBDeviceClaim) (*devicev1beta1.USBDeviceClaim, error) {
	return p().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (p USBDeviceClaimsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return p().Delete(context.TODO(), name, *options)
}

func (p USBDeviceClaimsClient) List(opts metav1.ListOptions) (*devicev1beta1.USBDeviceClaimList, error) {
	return p().List(context.TODO(), opts)
}

func (p USBDeviceClaimsClient) Watch(metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (p USBDeviceClaimsClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *devicev1beta1.USBDeviceClaim, err error) {
	panic("implement me")
}

func (p USBDeviceClaimsClient) UpdateStatus(d *devicev1beta1.USBDeviceClaim) (*devicev1beta1.USBDeviceClaim, error) {
	return p().Update(context.TODO(), d, metav1.UpdateOptions{})
}

type USBDeviceClaimsCache func() v1beta1.USBDeviceClaimInterface

func (p USBDeviceClaimsCache) Get(name string) (*devicev1beta1.USBDeviceClaim, error) {
	return p().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p USBDeviceClaimsCache) List(labels.Selector) ([]*devicev1beta1.USBDeviceClaim, error) {
	usbcs, err := p().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*devicev1beta1.USBDeviceClaim, 0, len(usbcs.Items))
	for _, usb := range usbcs.Items {
		usb := usb
		result = append(result, &usb)
	}

	return result, nil
}

func (p USBDeviceClaimsCache) AddIndexer(_ string, _ devicesv1beta1ctl.USBDeviceClaimIndexer) {
	panic("implement me")
}

func (p USBDeviceClaimsCache) GetByIndex(indexName, key string) ([]*devicev1beta1.USBDeviceClaim, error) {
	switch indexName {
	case USBDeviceByAddress:
		var usbcs []*devicev1beta1.USBDeviceClaim

		usbcList, err := p.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}

		for _, usbc := range usbcList {
			usbc := usbc
			if fmt.Sprintf("%s-%s", usbc.Status.NodeName, usbc.Status.PCIAddress) == key {
				usbcs = append(usbcs, usbc)
			}
		}

		return usbcs, nil
	default:
		panic("implement me")
	}
}
