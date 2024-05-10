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
	ud, ok := obj.(*v1beta1.USBDevice)

	if ud == nil {
		return nil, nil
	}

	if !ok {
		logrus.Errorf("error casting object to USBDevice: %v", obj)
		return nil, nil
	}

	if ud.Status.NodeName == cl.nodeName {
		udcList, err := h.usbClaimClient.List(metav1.ListOptions{LabelSelector: cl.selector()})
		if err != nil {
			logrus.Errorf("error listing USBDeviceClaims during device watch: %v", err)
			return nil, err
		}

		var rr []relatedresource.Key
		for _, v := range udcList.Items {
			rr = append(rr, relatedresource.NewKey(v.Namespace, v.Name))
		}
		return rr, nil
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

	var (
		createList []*v1beta1.USBDevice
		updateList []*v1beta1.USBDevice
	)

	for _, localDevices := range localUSBDevices {
		for _, localUSBDevice := range localDevices {
			if existed, ok := mapStoredUSBDevices[localUSBDevice.DevicePath]; !ok {
				name := usbDeviceName(nodeName, localUSBDevice)
				createdOne := &v1beta1.USBDevice{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: cl.labels(),
					},
					Status: v1beta1.USBDeviceStatus{
						VendorID:     fmt.Sprintf("%04x", localUSBDevice.Vendor),
						ProductID:    fmt.Sprintf("%04x", localUSBDevice.Product),
						ResourceName: resourceName(name),
						NodeName:     nodeName,
						DevicePath:   localUSBDevice.DevicePath,
					},
				}
				createList = append(createList, createdOne)
			} else {
				existedCp := existed.DeepCopy()
				if isStatusChanged(existedCp, localUSBDevice) {
					existedCp.Status = v1beta1.USBDeviceStatus{
						VendorID:     fmt.Sprintf("%04x", localUSBDevice.Vendor),
						ProductID:    fmt.Sprintf("%04x", localUSBDevice.Product),
						ResourceName: resourceName(usbDeviceName(nodeName, localUSBDevice)),
						NodeName:     nodeName,
						DevicePath:   localUSBDevice.DevicePath,
					}
					updateList = append(updateList, existedCp)
				} else {
					delete(mapStoredUSBDevices, localUSBDevice.DevicePath)
				}
			}
		}
	}

	for _, usbDevice := range createList {
		createdOne := &v1beta1.USBDevice{
			ObjectMeta: metav1.ObjectMeta{
				Name:   usbDevice.Name,
				Labels: usbDevice.Labels,
			},
		}

		newOne, err := h.usbClient.Create(createdOne)
		if err != nil {
			logrus.Errorf("failed to create USB device: %v\n", err)
			return err
		}

		newOne.Status = usbDevice.Status
		if _, err = h.usbClient.UpdateStatus(newOne); err != nil {
			logrus.Errorf("failed to update new created USB device status: %v\n", err)
			return err
		}
	}

	for _, usbDevice := range updateList {
		if _, err := h.usbClient.UpdateStatus(usbDevice); err != nil {
			logrus.Errorf("failed to update existed USB device status: %v\n", err)
			return err
		}
	}

	// The left devices in mapStoredUSBDevices are not found in localUSBDevices, so we should delete them.
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

func resourceName(name string) string {
	return fmt.Sprintf("%s%s", KubeVirtResourcePrefix, name)
}
