package deviceplugins

import (
	"github.com/sirupsen/logrus"
	v1 "kubevirt.io/api/core/v1"
)

type LocalDevices struct {
	// For quicker indexing, map devices based on vendor string
	devices map[int][]*USBDevice
}

// finds by vendor and product
func (l *LocalDevices) find(vendor, product int) *USBDevice {
	if devices, exist := l.devices[vendor]; exist {
		for _, local := range devices {
			if local.Product == product {
				return local
			}
		}
	}
	return nil
}

// remove all cached elements
func (l *LocalDevices) remove(usbdevs []*USBDevice) {
	for _, dev := range usbdevs {
		devices, exists := l.devices[dev.Vendor]
		if !exists {
			continue
		}

		for i, usb := range devices {
			if usb.GetID() == dev.GetID() {
				devices = append(devices[:i], devices[i+1:]...)
				break
			}
		}

		l.devices[dev.Vendor] = devices
		if len(devices) == 0 {
			delete(l.devices, dev.Vendor)
		}
	}
}

// return a list of USBDevices while removing it from the list of local devices
func (l *LocalDevices) fetch(selectors []v1.USBSelector) ([]*USBDevice, bool) {
	usbdevs := make([]*USBDevice, 0, len(selectors))

	// we have to find all devices under this resource name
	for _, selector := range selectors {
		selector := selector
		vendor, product, err := parseSelector(&selector)
		if err != nil {
			logrus.Warningf("Failed to convert selector: %+v", selector)
			return nil, false
		}

		local := l.find(vendor, product)
		if local == nil {
			return nil, false
		}

		usbdevs = append(usbdevs, local)
	}

	// To avoid mapping the same usb device to different k8s plugins
	l.remove(usbdevs)
	return usbdevs, true
}
