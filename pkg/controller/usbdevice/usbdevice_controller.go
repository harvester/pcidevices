package usbdevice

import (
	"fmt"
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

func (dev *USBDevice) GetID() string {
	return fmt.Sprintf("%04x:%04x-%02d:%02d", dev.Vendor, dev.Product, dev.Bus, dev.DeviceNumber)
}

func NewHandler(usbClient ctlpcidevicerv1.USBDeviceController, virtClient kubecli.KubevirtClient) *Handler {
	return &Handler{
		usbClient:  usbClient,
		virtClient: virtClient,
	}
}

func (h *Handler) ReconcileUSBDevices(nodeName string) error {
	err, localUSBDevices := deviceplugins.WalkUSBDevices()
	if err != nil {
		return fmt.Errorf("failed to walk USB devices: %v", err)
	}

	storedUSBDevices, err := h.usbClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list USB devices: %v", err)
	}

	mapStoredUSBDevices := make(map[string]*v1beta1.USBDevice)
	for _, storedUSBDevice := range storedUSBDevices.Items {
		mapStoredUSBDevices[storedUSBDevice.Status.DevicePath] = &storedUSBDevice
	}

	for vendorId, localDevices := range localUSBDevices {
		for _, localUSBDevice := range localDevices {
			if existed, ok := mapStoredUSBDevices[localUSBDevice.DevicePath]; !ok {
				name := usbDeviceName(nodeName, localUSBDevice, vendorId)
				newOne, err := h.usbClient.Create(&v1beta1.USBDevice{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
						Labels: map[string]string{
							"nodename": nodeName,
						},
					},
				})

				if err != nil {
					return fmt.Errorf("failed to create USB device: %v", err)
				}

				newOne.Status = v1beta1.USBDeviceStatus{
					VendorID:     fmt.Sprintf("%04x", localUSBDevice.Vendor),
					ProductID:    fmt.Sprintf("%04x", localUSBDevice.Product),
					ResourceName: fmt.Sprintf("kubevirt.io/%s", name),
					NodeName:     nodeName,
					DevicePath:   localUSBDevice.DevicePath,
				}

				fmt.Printf("USBDevice old: %#v\n", newOne)
				newOne, err = h.usbClient.UpdateStatus(newOne)
				if err != nil {
					return fmt.Errorf("failed to update USB device status: %v", err)
				}
				fmt.Printf("USBDevice new: %#v\n", newOne)
			} else {
				if isStatusChanged(existed, localUSBDevice) {
					existed.Status.VendorID = fmt.Sprintf("%04x", localUSBDevice.Vendor)
					existed.Status.ProductID = fmt.Sprintf("%04x", localUSBDevice.Product)

					existed, err = h.usbClient.UpdateStatus(existed)
					if err != nil {
						return fmt.Errorf("failed to update existed USB device status: %v", err)
					}
				} else {
					delete(mapStoredUSBDevices, localUSBDevice.DevicePath)
				}
			}
		}
	}

	for _, usbDevice := range mapStoredUSBDevices {
		if err := h.usbClient.Delete(usbDevice.Name, &metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete USB device: %v", err)
		}
	}

	return nil
}

func usbDeviceName(nodeName string, localUSBDevice *deviceplugins.USBDevice, vendorId int) string {
	devicePath := strings.Replace(localUSBDevice.DevicePath, "/dev/bus/usb/", "", -1)
	devicePath = strings.Join(strings.Split(devicePath, "/"), "")
	name := fmt.Sprintf("%s-%04x-%04x-%s", nodeName, vendorId, localUSBDevice.Product, devicePath)
	return name
}

func isStatusChanged(existed *v1beta1.USBDevice, localUSBDevice *deviceplugins.USBDevice) bool {
	return existed.Status.VendorID != fmt.Sprintf("%04x", localUSBDevice.Vendor) ||
		existed.Status.ProductID != fmt.Sprintf("%04x", localUSBDevice.Product)
}
