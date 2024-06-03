package usbdevice

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctldevicerv1vbeta1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/gousb"
	"github.com/harvester/pcidevices/pkg/util/gousb/usbid"
)

type DevHandler struct {
	usbClient      ctldevicerv1vbeta1.USBDeviceClient
	usbClaimClient ctldevicerv1vbeta1.USBDeviceClaimClient
	usbCache       ctldevicerv1vbeta1.USBDeviceCache
	usbClaimCache  ctldevicerv1vbeta1.USBDeviceClaimCache
}

var walkUSBDevices = deviceplugins.WalkUSBDevices

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

func NewHandler(
	usbClient ctldevicerv1vbeta1.USBDeviceClient,
	usbClaimClient ctldevicerv1vbeta1.USBDeviceClaimClient,
	usbCache ctldevicerv1vbeta1.USBDeviceCache,
	usbClaimCache ctldevicerv1vbeta1.USBDeviceClaimCache,
) *DevHandler {
	return &DevHandler{
		usbClient:      usbClient,
		usbClaimClient: usbClaimClient,
		usbCache:       usbCache,
		usbClaimCache:  usbClaimCache,
	}
}

func (h *DevHandler) OnDeviceChange(_ string, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	ud, ok := obj.(*v1beta1.USBDevice)

	if ud == nil {
		return nil, nil
	}

	if !ok {
		logrus.Errorf("error casting object to USBDevice: %v", obj)
		return nil, nil
	}

	if ud.Status.NodeName == cl.nodeName {
		udcList, err := h.usbClaimCache.List(labels.SelectorFromSet(cl.labels()))
		if err != nil {
			logrus.Errorf("error listing USBDeviceClaims during device watch: %v", err)
			return nil, err
		}

		var rr []relatedresource.Key
		for _, v := range udcList {
			if v.Status.PCIAddress == ud.Status.PCIAddress {
				rr = append(rr, relatedresource.NewKey(v.Namespace, v.Name))
			}
		}
		return rr, nil
	}

	return nil, nil
}

func (h *DevHandler) ReconcileUSBDevices() error {
	nodeName := cl.nodeName

	localUSBDevices, err := walkUSBDevices()
	if err != nil {
		logrus.Errorf("failed to walk USB devices: %v\n", err)
		return err
	}

	storedUSBDevices, err := h.usbCache.List(labels.SelectorFromSet(cl.labels()))
	if err != nil {
		logrus.Errorf("failed to list USB devices: %v\n", err)
		return err
	}

	mapStoredUSBDevices := make(map[string]*v1beta1.USBDevice)
	for _, storedUSBDevice := range storedUSBDevices {
		storedUSBDevice := storedUSBDevice
		mapStoredUSBDevices[storedUSBDevice.Status.DevicePath] = storedUSBDevice
	}

	createList, updateList := h.getList(localUSBDevices, mapStoredUSBDevices, nodeName)

	err = h.handleList(createList, updateList, mapStoredUSBDevices)
	if err != nil {
		return err
	}

	return nil
}

func (h *DevHandler) handleList(createList []*v1beta1.USBDevice, updateList []*v1beta1.USBDevice, mapStoredUSBDevices map[string]*v1beta1.USBDevice) error {
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
		if usbDevice.Status.Enabled {
			logrus.Warningf("USB device %s is still enabled, but it's not discovered in local usb devices. Please check your node could detect that usb device, skippping delete.\n", usbDevice.Name)
			continue
		}

		if err := h.usbClient.Delete(usbDevice.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("failed to delete USB device: %v\n", err)
			return err
		}
	}

	return nil
}

func (h *DevHandler) getList(localUSBDevices map[int][]*deviceplugins.USBDevice, mapStoredUSBDevices map[string]*v1beta1.USBDevice, nodeName string) ([]*v1beta1.USBDevice, []*v1beta1.USBDevice) {
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
						Description:  usbid.DescribeWithVendorAndProduct(gousb.ID(localUSBDevice.Vendor), gousb.ID(localUSBDevice.Product)),
						PCIAddress:   localUSBDevice.PCIAddress,
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
						Description:  usbid.DescribeWithVendorAndProduct(gousb.ID(localUSBDevice.Vendor), gousb.ID(localUSBDevice.Product)),
						PCIAddress:   localUSBDevice.PCIAddress,
					}
					updateList = append(updateList, existedCp)
				} else {
					delete(mapStoredUSBDevices, localUSBDevice.DevicePath)
				}
			}
		}
	}

	return createList, updateList
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
