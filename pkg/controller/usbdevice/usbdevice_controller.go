package usbdevice

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctlpcidevicerv1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type Handler struct {
	usbClient      ctlpcidevicerv1.USBDeviceController
	usbClaimClient ctlpcidevicerv1.USBDeviceClaimController
	virtClient     kubecli.KubevirtClient
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

func NewHandler(usbClient ctlpcidevicerv1.USBDeviceController, usbClaimClient ctlpcidevicerv1.USBDeviceClaimController, virtClient kubecli.KubevirtClient) *Handler {
	return &Handler{
		usbClient:      usbClient,
		usbClaimClient: usbClaimClient,
		virtClient:     virtClient,
	}
}

func (h *Handler) OnDeviceChange(_ string, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if ud, ok := obj.(*v1beta1.USBDevice); ok {
		logrus.Infof("USBDevice %s changed", ud.Name)
		logrus.Infof(ud.Status.NodeName, cl.nodeName)
		if ud.Status.NodeName == cl.nodeName {
			udcList, err := h.usbClaimClient.List(metav1.ListOptions{LabelSelector: cl.selector()})
			if err != nil {
				logrus.Errorf("error listing USBDeviceClaims during device watch: %v", err)
				return nil, err
			}
			var rr []relatedresource.Key
			for _, v := range udcList.Items {
				fmt.Println("****related resource key", v.Namespace, v.Name)
				rr = append(rr, relatedresource.NewKey(v.Namespace, v.Name))
			}
			return rr, nil
		}
	} else {
		logrus.Errorf("error casting object to USBDevice: %v", obj)
	}

	return nil, nil
}

func (h *Handler) ReconcileUSBDevices() error {
	nodeName := cl.nodeName

	err, localUSBDevices := deviceplugins.WalkUSBDevices()
	if err != nil {
		logrus.Errorf("failed to walk USB devices: %v\n", err)
		return err
	}

	storedUSBDevices, err := h.usbClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("failed to list USB devices: %v\n", err)
		return err
	}

	mapStoredUSBDevices := make(map[string]v1beta1.USBDevice)
	for _, storedUSBDevice := range storedUSBDevices.Items {
		mapStoredUSBDevices[storedUSBDevice.Status.DevicePath] = storedUSBDevice
	}

	for _, localDevices := range localUSBDevices {
		for _, localUSBDevice := range localDevices {
			if existed, ok := mapStoredUSBDevices[localUSBDevice.DevicePath]; !ok {
				name := usbDeviceName(nodeName, localUSBDevice)
				newOne, err := h.usbClient.Create(&v1beta1.USBDevice{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: cl.labels(),
					},
				})

				if err != nil {
					logrus.Errorf("failed to create USB device: %v\n", err)
					return err
				}

				newOne.Status = v1beta1.USBDeviceStatus{
					VendorID:     fmt.Sprintf("%04x", localUSBDevice.Vendor),
					ProductID:    fmt.Sprintf("%04x", localUSBDevice.Product),
					ResourceName: fmt.Sprintf("kubevirt.io/%s", name),
					NodeName:     nodeName,
					DevicePath:   localUSBDevice.DevicePath,
				}

				newOne, err = h.usbClient.UpdateStatus(newOne)
				if err != nil {
					logrus.Errorf("failed to update USB device status: %v\n", err)
					return err
				}
			} else {
				existedCp := existed.DeepCopy()

				if isStatusChanged(existedCp, localUSBDevice) {
					existedCp.Status.VendorID = fmt.Sprintf("%04x", localUSBDevice.Vendor)
					existedCp.Status.ProductID = fmt.Sprintf("%04x", localUSBDevice.Product)
					existedCp.Status.ResourceName = fmt.Sprintf("kubevirt.io/%s", usbDeviceName(nodeName, localUSBDevice))
					existedCp.Name = usbDeviceName(nodeName, localUSBDevice)

					_, err = h.usbClient.UpdateStatus(existedCp)
					if err != nil {
						logrus.Errorf("failed to update existed USB device status: %v\n", err)
						return err
					}
				} else {
					delete(mapStoredUSBDevices, localUSBDevice.DevicePath)
				}
			}
		}
	}

	for _, usbDevice := range mapStoredUSBDevices {
		if err := h.usbClient.Delete(usbDevice.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("failed to delete USB device: %v\n", err)
			return err
		}
	}

	return nil
}

func usbDeviceName(nodeName string, localUSBDevice *deviceplugins.USBDevice) string {
	devicePath := strings.Replace(localUSBDevice.DevicePath, "/dev/bus/usb/", "", -1)
	devicePath = strings.Join(strings.Split(devicePath, "/"), "")
	name := fmt.Sprintf("%s-%04x-%04x-%s", nodeName, localUSBDevice.Vendor, localUSBDevice.Product, devicePath)
	return name
}

func isStatusChanged(existed *v1beta1.USBDevice, localUSBDevice *deviceplugins.USBDevice) bool {
	return existed.Status.VendorID != fmt.Sprintf("%04x", localUSBDevice.Vendor) ||
		existed.Status.ProductID != fmt.Sprintf("%04x", localUSBDevice.Product)
}
