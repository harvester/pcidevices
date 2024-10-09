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

	reconcileSignal chan struct{}
}

type UsageList struct {
	createList   []*v1beta1.USBDevice
	updateList   []*v1beta1.USBDevice
	deleteList   []*v1beta1.USBDevice
	orphanedList []*v1beta1.USBDevice
}

var walkUSBDevices = deviceplugins.WalkUSBDevices

func NewHandler(
	usbClient ctldevicerv1vbeta1.USBDeviceClient,
	usbClaimClient ctldevicerv1vbeta1.USBDeviceClaimClient,
	usbCache ctldevicerv1vbeta1.USBDeviceCache,
	usbClaimCache ctldevicerv1vbeta1.USBDeviceClaimCache,
) *DevHandler {
	return &DevHandler{
		usbClient:       usbClient,
		usbClaimClient:  usbClaimClient,
		usbCache:        usbCache,
		usbClaimCache:   usbClaimCache,
		reconcileSignal: make(chan struct{}, 1),
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
			case <-orChan(watcher.Events, h.reconcileSignal):
				// we need reconcile whatever there is a change in /dev/bus/usb/xxx or reconcile signal is received
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

func (h *DevHandler) handleList(usageList UsageList) error {
	for _, usbDevice := range usageList.createList {
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

	for _, usbDevice := range usageList.updateList {
		if _, err := h.usbClient.UpdateStatus(usbDevice); err != nil {
			logrus.Errorf("failed to update existed USB device status: %v\n", err)
			return err
		}
	}

	for _, usbDevice := range usageList.orphanedList {
		err := h.usbClaimClient.Delete(usbDevice.Name, &metav1.DeleteOptions{})

		if err == nil {
			continue
		}

		logrus.Errorf("failed to delete orphaned device claim: %v\n", err)
		usbDevice.Status.Status = v1beta1.USBDeviceStatusOrphaned
		usbDevice.Status.Message = "The USB device is orphaned, please remove it from virtual machine and disable it."

		_, err = h.usbClient.UpdateStatus(usbDevice)
		if err != nil {
			logrus.Errorf("failed to update orphaned USB device status: %v\n", err)
			continue
		}
	}

	for _, usbDevice := range usageList.deleteList {
		if err := h.usbClient.Delete(usbDevice.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("failed to delete USB device: %v\n", err)
			return err
		}
	}

	return nil
}

func (h *DevHandler) getList(localUSBDevices map[int][]*deviceplugins.USBDevice, mapStoredUSBDevices map[string]*v1beta1.USBDevice, nodeName string) UsageList {
	var (
		createList   []*v1beta1.USBDevice
		updateList   []*v1beta1.USBDevice
		orphanedList []*v1beta1.USBDevice
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
			// This case is for some users might directly remove USB device without disabling `usbdeviceclaim`.
			// Then re-plugging the USB device will change the status.devicePath,
			// so we're not able to use original devicePath to find the device.
			// Those USB device became orphaned, we should delete those `usbdevicecalim`.
			orphanedList = append(orphanedList, usbDevice)
		}

		deleteList = append(deleteList, usbDevice)
	}

	return UsageList{
		createList:   createList,
		updateList:   updateList,
		deleteList:   deleteList,
		orphanedList: orphanedList,
	}
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

func orChan[T any, V any](ch1 chan T, ch2 <-chan V) <-chan struct{} {
	orDone := make(chan struct{})
	go func() {
		defer close(orDone)
		select {
		case <-ch1:
		case <-ch2:
		}
	}()
	return orDone
}
