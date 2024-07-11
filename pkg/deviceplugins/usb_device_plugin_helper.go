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
}

func (dev *USBDevice) GetID() string {
	return fmt.Sprintf("%04x:%04x-%02d:%02d", dev.Vendor, dev.Product, dev.Bus, dev.DeviceNumber)
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
