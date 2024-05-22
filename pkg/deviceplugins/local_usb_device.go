package deviceplugins

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
