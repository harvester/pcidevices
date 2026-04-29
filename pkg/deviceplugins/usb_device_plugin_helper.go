package deviceplugins

/* This file was part of the KubeVirt project, copied to this project
 * to get around private package issues.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2024 SUSE, LLC.
 *
 */

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// usbClassNames maps USB base class codes (from https://www.usb.org/defined-class-codes)
// to human-readable names.
var usbClassNames = map[int]string{
	0x01: "Audio",
	0x02: "Communications",
	0x03: "HID",
	0x05: "Physical",
	0x06: "Image",
	0x07: "Printer",
	0x08: "Mass Storage",
	0x09: "Hub",
	0x0A: "CDC-Data",
	0x0B: "Smart Card",
	0x0D: "Content Security",
	0x0E: "Video",
	0x0F: "Personal Healthcare",
	0x10: "Audio/Video",
	0x11: "Billboard",
	0x12: "USB Type-C Bridge",
	0x13: "USB Bulk Display Protocol",
	0x14: "MCTP over USB",
	0x3C: "I3C Device",
	0xDC: "Diagnostic",
	0xE0: "Wireless Controller",
	0xEF: "Miscellaneous",
	0xFE: "Application Specific",
	0xFF: "Vendor Specific",
}

type USBDevice struct {
	Name         string
	Manufacturer string
	Vendor       int
	Product      int
	BCD          int
	Bus          int
	DeviceNumber int
	Serial       string
	DevicePath   string
	PCIAddress   string
	ClassType    string
	ProductName  string
}

func (dev *USBDevice) GetID() string {
	return fmt.Sprintf("%04x:%04x-%02d:%02d", dev.Vendor, dev.Product, dev.Bus, dev.DeviceNumber)
}

// readSysfsString reads a plain-text sysfs file (e.g. "product", "manufacturer")
// and returns its trimmed content, or an empty string on error.
func readSysfsString(dir, filename string) string {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// parseClassCode reads a sysfs file containing a hex USB class code (e.g. "bDeviceClass")
// and returns the parsed integer value.
// Returns (0, false) on any read or parse error.
func parseClassCode(dir, filename string) (int, bool) {
	data := readSysfsString(dir, filename)
	if data == "" {
		return 0, false
	}

	// Convert the sysfs hex string to a decimal integer, then look it up in usbClassNames.
	// e.g. "09" (hex string) → 9 (decimal int) → usbClassNames[9] = "Hub"
	//      "ff" (hex string) → 255 (decimal int) → usbClassNames[255] = "Vendor Specific"
	code, err := strconv.ParseInt(strings.TrimSpace(string(data)), 16, 32)
	if err != nil {
		return 0, false
	}

	return int(code), true
}

// classTypeName returns the human-readable name for a USB class code,
// falling back to "Unknown (0xNN)" if the code is not in the map.
func classTypeName(code int) string {
	if name, ok := usbClassNames[code]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (0x%02x)", code)
}

// parseUSBClassType determines the USB class name for a device rooted at path.
// When bDeviceClass is 00, the class is reported per-interface; in that case the
// function reads bInterfaceClass from the first interface sub-directory.
func parseUSBClassType(path string) string {
	code, ok := parseClassCode(path, "bDeviceClass")
	if !ok {
		logrus.Debugf("Unable to read or parse bDeviceClass from %s", path)
		return ""
	}

	if code != 0 {
		return classTypeName(code)
	}

	// class 00 means "use Interface Descriptors" – look at the first interface sub-directory
	entries, err := os.ReadDir(path)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.Contains(entry.Name(), ":") {
			continue
		}

		if iCode, ok := parseClassCode(filepath.Join(path, entry.Name()), "bInterfaceClass"); ok {
			return classTypeName(iCode)
		}
	}

	return ""
}

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

	u.ProductName = readSysfsString(path, "product")
	u.ClassType = parseUSBClassType(path)

	return &u
}

func parseSysUeventKeyValue(key string, value string, u *USBDevice) bool {
	switch key {
	case "BUSNUM":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			logrus.Errorf("Unable to parse BUSNUM %s\n", value)
			return false
		}
		u.Bus = int(val)
	case "DEVNUM":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			logrus.Errorf("Unable to parse DEVNUM %s\n", value)
			return false
		}
		u.DeviceNumber = int(val)
	case "PRODUCT":
		products := strings.Split(value, "/")
		if len(products) != 3 {
			logrus.Errorf("PRODUCT value %s is not in the format of xx/xx/xx\n", value)
			return false
		}

		val, err := strconv.ParseInt(products[0], 16, 32)
		if err != nil {
			logrus.Errorf("Unable to parse PRODUCT[0] %s\n", value)
			return false
		}
		u.Vendor = int(val)

		val, err = strconv.ParseInt(products[1], 16, 32)
		if err != nil {
			logrus.Errorf("Unable to parse PRODUCT[1] %s\n", value)
			return false
		}
		u.Product = int(val)

		val, err = strconv.ParseInt(products[2], 16, 32)
		if err != nil {
			logrus.Errorf("Unable to parse PRODUCT[2] %s\n", value)
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
