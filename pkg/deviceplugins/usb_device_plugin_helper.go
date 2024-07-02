package deviceplugins

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "kubevirt.io/api/core/v1"
)

func parseSysUeventFile(path string) *USBDevice {
	link, err := os.Readlink(path)
	if err != nil {
		return nil
	}

	// Grab all details we are interested from uevent
	file, err := os.Open(filepath.Join(path, "uevent"))
	if err != nil {
		logrus.Printf("Unable to access %s/%s\n", path, "uevent")
		return nil
	}
	defer file.Close()

	u := USBDevice{
		PCIAddress: parseUSBSymLinkToPCIAddress(link),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, "=")
		if len(values) != 2 {
			logrus.Printf("Skipping %s due not being key=value\n", line)
			continue
		}

		key, value := values[0], values[1]
		if !parseSysUeventKeyValue(key, value, &u) {
			return nil
		}
	}

	return &u
}

func parseSysUeventKeyValue(key string, value string, u *USBDevice) bool {
	switch key {
	case "BUSNUM":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return false
		}
		u.Bus = int(val)
	case "DEVNUM":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return false
		}
		u.DeviceNumber = int(val)
	case "PRODUCT":
		products := strings.Split(value, "/")
		if len(products) != 3 {
			return false
		}

		val, err := strconv.ParseInt(products[0], 16, 32)
		if err != nil {
			return false
		}
		u.Vendor = int(val)

		val, err = strconv.ParseInt(products[1], 16, 32)
		if err != nil {
			return false
		}
		u.Product = int(val)

		val, err = strconv.ParseInt(products[2], 16, 32)
		if err != nil {
			return false
		}
		u.BCD = int(val)
	case "DEVNAME":
		u.DevicePath = filepath.Join("/dev", value)
	default:
		logrus.Printf("Skipping unknown key=value %s=%s\n", key, value)
	}

	return true
}

func WalkUSBDevices() (map[int][]*USBDevice, error) {
	usbDevices := make(map[int][]*USBDevice, 0)
	err := filepath.Walk("/sys/bus/usb/devices", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore named usb controllers
		if strings.HasPrefix(info.Name(), "usb") {
			return nil
		}
		// We are interested in actual USB devices information that
		// contains idVendor and idProduct. We can skip all others.
		if _, err := os.Stat(filepath.Join(path, "idVendor")); err != nil {
			return nil
		}

		// Get device information
		if device := parseSysUeventFile(path); device != nil {
			usbDevices[device.Vendor] = append(usbDevices[device.Vendor], device)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return usbDevices, nil
}

func parseUSBSymLinkToPCIAddress(link string) string {
	paths := strings.Split(link, "/usb")

	if len(paths) < 2 {
		return ""
	}

	paths = strings.Split(paths[0], "/")

	return paths[len(paths)-1]
}

func discoverPluggedUSBDevices() *LocalDevices {
	usbDevices := make(map[int][]*USBDevice, 0)
	err := filepath.Walk(pathToUSBDevices, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Ignore named usb controllers
		if strings.HasPrefix(info.Name(), "usb") {
			return nil
		}
		// We are interested in actual USB devices information that
		// contains idVendor and idProduct. We can skip all others.
		if _, err := os.Stat(filepath.Join(path, "idVendor")); err != nil {
			return nil
		}

		// Get device information
		if device := parseSysUeventFile(path); device != nil {
			usbDevices[device.Vendor] = append(usbDevices[device.Vendor], device)
		}
		return nil
	})

	if err != nil {
		logrus.Error("Failed when walking usb devices tree")
	}
	return &LocalDevices{devices: usbDevices}
}

func parseSelector(s *v1.USBSelector) (int, int, error) {
	val, err := strconv.ParseInt(s.Vendor, 16, 32)
	if err != nil {
		return -1, -1, err
	}
	vendor := int(val)

	val, err = strconv.ParseInt(s.Product, 16, 32)
	if err != nil {
		return -1, -1, err
	}
	product := int(val)

	return vendor, product, nil
}

func DiscoverAllowedUSBDevices(usbs []v1.USBHostDevice) map[string][]*PluginDevices {
	// The return value: USB USBDevice Plugins found and permitted to be exposed
	plugins := make(map[string][]*PluginDevices)
	// All USB devices found plugged in the Node
	localDevices := discoverLocalUSBDevicesFunc()
	for _, usbConfig := range usbs {
		resourceName := usbConfig.ResourceName
		// only accept ExternalResourceProvider: true for USB devices
		if !usbConfig.ExternalResourceProvider {
			logrus.Errorf("Skipping discovery of %s. To be handled by kubevirt internally",
				resourceName)
			continue
		}
		index := 0
		usbdevs, foundAll := localDevices.fetch(usbConfig.Selectors)
		for foundAll {
			// Create new USB USBDevice Plugin with found USB Devices for this resource name
			pluginDevices := newPluginDevices(resourceName, index, usbdevs)
			plugins[resourceName] = append(plugins[resourceName], pluginDevices)
			index++
			usbdevs, foundAll = localDevices.fetch(usbConfig.Selectors)
		}
	}
	return plugins
}
