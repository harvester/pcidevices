package usbdevice

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
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
		udcList, err := h.usbClaimCache.GetByIndex(v1beta1.USBDevicePCIAddress, ud.Status.PCIAddress)
		if err != nil {
			logrus.Errorf("error listing USBDeviceClaims during device watch: %v", err)
			return nil, err
		}

		var rr []relatedresource.Key
		for _, v := range udcList {
			rr = append(rr, relatedresource.NewKey(v.Namespace, v.Name))
		}
		return rr, nil
	}

	return nil, nil
}

func (h *DevHandler) WatchUSBDevices(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to creating a fsnotify watcher: %v", err)
	}

	if err := filepath.WalkDir("/dev/bus/usb/", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk /dev/bus/usb: %v", err)
		}
		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				return fmt.Errorf("failed to watch device %s parent directory: %s", path, err)
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk /dev/bus/usb: %v", err)
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}

				// we need reconcile whatever there is a change in /dev/bus/usb/xxx
				if err := h.reconcile(); err != nil {
					logrus.Errorf("failed to reconcile USB devices: %v", err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				logrus.Errorf("fsnotify watcher error: %v", err)
			}
		}
	}()

	return nil
}

func (h *DevHandler) reconcile() error {
	nodeName := cl.nodeName

	localUSBDevices, err := walkUSBDevices()
	if err != nil {
		logrus.Errorf("failed to walk USB devices: %v\n", err)
		return err
	}

	list, err := h.usbClient.List(metav1.ListOptions{LabelSelector: labels.Set(cl.labels()).String()})
	if err != nil {
		logrus.Errorf("failed to list USB devices: %v\n", err)
		return err
	}

	mapStoredUSBDevices := make(map[string]*v1beta1.USBDevice)
	for _, storedUSBDevice := range list.Items {
		storedUSBDevice := storedUSBDevice
		mapStoredUSBDevices[storedUSBDevice.Status.DevicePath] = &storedUSBDevice
	}

	err = h.handleList(h.getList(localUSBDevices, mapStoredUSBDevices, nodeName))
	if err != nil {
		return err
	}

	return nil
}

func (h *DevHandler) handleList(createList []*v1beta1.USBDevice, updateList []*v1beta1.USBDevice, deleteList []*v1beta1.USBDevice) error {
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

	for _, usbDevice := range deleteList {
		if err := h.usbClient.Delete(usbDevice.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("failed to delete USB device: %v\n", err)
			return err
		}
	}

	return nil
}

func (h *DevHandler) getList(localUSBDevices map[int][]*deviceplugins.USBDevice, mapStoredUSBDevices map[string]*v1beta1.USBDevice, nodeName string) ([]*v1beta1.USBDevice, []*v1beta1.USBDevice, []*v1beta1.USBDevice) {
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
				}
				delete(mapStoredUSBDevices, localUSBDevice.DevicePath)
			}
		}
	}

	deleteList := make([]*v1beta1.USBDevice, 0, len(mapStoredUSBDevices))

	// The left devices in mapStoredUSBDevices are not found in localUSBDevices, so we should delete them.
	for _, usbDevice := range mapStoredUSBDevices {
		usbDevice := usbDevice
		if usbDevice.Status.Enabled {
			logrus.Warningf("USB device %s is still enabled, but it's not discovered in local usb devices. Please check your node could detect that usb device, skippping delete.\n", usbDevice.Name)
			continue
		}

		deleteList = append(deleteList, usbDevice)
	}

	return createList, updateList, deleteList
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
