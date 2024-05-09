package usbdevice

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctlpcidevicerv1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type Handler struct {
	usbClient  ctlpcidevicerv1.USBDeviceController
	virtClient kubecli.KubevirtClient
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
}

func NewHandler(usbClient ctlpcidevicerv1.USBDeviceController, virtClient kubecli.KubevirtClient) *Handler {
	return &Handler{
		usbClient:  usbClient,
		virtClient: virtClient,
	}
}

func (dev *USBDevice) GetID() string {
	return fmt.Sprintf("%04x:%04x-%02d:%02d", dev.Vendor, dev.Product, dev.Bus, dev.DeviceNumber)
}

func parseSysUeventFile(path string) *USBDevice {
	// Grab all details we are interested from uevent
	file, err := os.Open(filepath.Join(path, "uevent"))
	if err != nil {
		fmt.Printf("Unable to access %s/%s\n", path, "uevent")
		return nil
	}
	defer file.Close()

	u := USBDevice{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, "=")
		if len(values) != 2 {
			fmt.Printf("Skipping %s due not being key=value\n", line)
			continue
		}
		switch values[0] {
		case "BUSNUM":
			val, err := strconv.ParseInt(values[1], 10, 32)
			if err != nil {
				return nil
			}
			u.Bus = int(val)
		case "DEVNUM":
			val, err := strconv.ParseInt(values[1], 10, 32)
			if err != nil {
				return nil
			}
			u.DeviceNumber = int(val)
		case "PRODUCT":
			products := strings.Split(values[1], "/")
			if len(products) != 3 {
				return nil
			}

			val, err := strconv.ParseInt(products[0], 16, 32)
			if err != nil {
				return nil
			}
			u.Vendor = int(val)

			val, err = strconv.ParseInt(products[1], 16, 32)
			if err != nil {
				return nil
			}
			u.Product = int(val)

			val, err = strconv.ParseInt(products[2], 16, 32)
			if err != nil {
				return nil
			}
			u.BCD = int(val)
		case "DEVNAME":
			u.DevicePath = filepath.Join("/dev", values[1])
		default:
			fmt.Printf("Skipping unhandled line: %s\n", line)
		}
	}
	return &u
}

func (h *Handler) ReconcileUSBDevices() {
	err, usbDevices := deviceplugins.WalkUSBDevices()
	if err != nil {
		fmt.Println(usbDevices)
		fmt.Println("========")
		fmt.Println(err)
		fmt.Println("========")
	}

	for vendorId, usbDevices := range usbDevices {
		for _, usbDevice := range usbDevices {
			nodeName := os.Getenv("NODE_NAME")
			devicePath := strings.Replace(usbDevice.DevicePath, "/dev/bus/usb/", "", -1)
			devicePath = strings.Join(strings.Split(devicePath, "/"), "")
			name := fmt.Sprintf("%s-%04x-%04x-%s", nodeName, vendorId, usbDevice.Product, devicePath)

			fmt.Println(fmt.Sprintf("%04x", usbDevice.Vendor))
			fmt.Println(fmt.Sprintf("%04x", usbDevice.Product))
			fmt.Println(fmt.Sprintf("kubevirt.io/%s", name))
			fmt.Println(nodeName)
			fmt.Println(usbDevice.DevicePath)

			if newOne, err := h.usbClient.Create(&v1beta1.USBDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}); err != nil {
				fmt.Println(err)
			} else {
				newOne.Status = v1beta1.USBDeviceStatus{
					VendorID:     fmt.Sprintf("%04x", usbDevice.Vendor),
					ProductID:    fmt.Sprintf("%04x", usbDevice.Product),
					ResourceName: fmt.Sprintf("kubevirt.io/%s", name),
					NodeName:     nodeName,
					DevicePath:   usbDevice.DevicePath,
				}
				fmt.Printf("USBDevice old: %#v\n", newOne)
				newOne, err = h.usbClient.UpdateStatus(newOne)
				fmt.Printf("USBDevice new: %#v\n", newOne)
			}
		}
	}
}
