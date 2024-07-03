package usbdevice

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctldevicerv1beta1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/pcidevices/pkg/generated/controllers/kubevirt.io/v1"
)

var (
	discoverAllowedUSBDevices = deviceplugins.DiscoverAllowedUSBDevices
)

type DevClaimHandler struct {
	usbClaimClient        ctldevicerv1beta1.USBDeviceClaimClient
	usbClient             ctldevicerv1beta1.USBDeviceClient
	virtClient            ctlkubevirtv1.KubeVirtClient
	lock                  *sync.Mutex
	usbDeviceCache        ctldevicerv1beta1.USBDeviceCache
	devicePlugin          map[string]*deviceController
	devicePluginConvertor devicePluginConvertor
}

type deviceController struct {
	device  deviceplugins.USBDevicePluginInterface
	stop    chan struct{}
	started bool
}

type devicePluginConvertor func(resourceName string, devices []*deviceplugins.PluginDevices) deviceplugins.USBDevicePluginInterface

func NewClaimHandler(
	usbDeviceCache ctldevicerv1beta1.USBDeviceCache,
	usbClaimClient ctldevicerv1beta1.USBDeviceClaimClient,
	usbClient ctldevicerv1beta1.USBDeviceClient,
	virtClient ctlkubevirtv1.KubeVirtClient,
	devicePluginHelper devicePluginConvertor,
) *DevClaimHandler {
	return &DevClaimHandler{
		usbDeviceCache:        usbDeviceCache,
		usbClaimClient:        usbClaimClient,
		usbClient:             usbClient,
		virtClient:            virtClient,
		lock:                  &sync.Mutex{},
		devicePlugin:          map[string]*deviceController{},
		devicePluginConvertor: devicePluginHelper,
	}
}

func (h *DevClaimHandler) OnUSBDeviceClaimChanged(_ string, usbDeviceClaim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if usbDeviceClaim == nil || usbDeviceClaim.DeletionTimestamp != nil {
		return usbDeviceClaim, nil
	}

	if usbDeviceClaim.OwnerReferences == nil {
		err := fmt.Errorf("usb device claim %s has no owner reference", usbDeviceClaim.Name)
		logrus.Error(err)
		return usbDeviceClaim, err
	}

	usbDevice, err := h.usbDeviceCache.Get(usbDeviceClaim.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Errorf("usb device %s not found", usbDeviceClaim.Name)
			return usbDeviceClaim, nil
		}
		return usbDeviceClaim, err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	virt, err := h.virtClient.Get(KubeVirtNamespace, KubeVirtResource, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	_, err = h.updateKubeVirt(virt, usbDevice)
	if err != nil {
		logrus.Errorf("failed to update kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	// start device plugin if it's not started yet.
	if _, ok := h.devicePlugin[usbDeviceClaim.Name]; !ok {
		pluginDevices := discoverAllowedUSBDevices(convertToKubeVirtUSBFormat(usbDevice))

		if pluginDevice := h.findDevicePlugin(pluginDevices, usbDevice); pluginDevice != nil {
			usbDevicePlugin := h.devicePluginConvertor(usbDevice.Status.ResourceName, []*deviceplugins.PluginDevices{pluginDevice})
			deviceHan := &deviceController{
				device: usbDevicePlugin,
			}
			h.devicePlugin[usbDeviceClaim.Name] = deviceHan
			h.startDevicePlugin(deviceHan, usbDeviceClaim.Name)
		} else {
			logrus.Errorf("failed to find device plugin for usb device %s", usbDevice.Name)
			return usbDeviceClaim, err
		}
	}

	if !usbDevice.Status.Enabled {
		usbDeviceCp := usbDevice.DeepCopy()
		usbDeviceCp.Status.Enabled = true
		if _, err = h.usbClient.UpdateStatus(usbDeviceCp); err != nil {
			logrus.Errorf("failed to enable usb device %s status: %v", usbDeviceCp.Name, err)
			return usbDeviceClaim, err
		}
	}

	// just sync usb device pci address to usb device claim
	usbDeviceClaimCp := usbDeviceClaim.DeepCopy()
	usbDeviceClaimCp.Status.PCIAddress = usbDevice.Status.PCIAddress
	usbDeviceClaimCp.Status.NodeName = usbDevice.Status.NodeName

	return h.usbClaimClient.UpdateStatus(usbDeviceClaimCp)
}

func (h *DevClaimHandler) startDevicePlugin(deviceHan *deviceController, deviceName string) {
	if deviceHan.started {
		return
	}

	deviceHan.stop = make(chan struct{})

	go func() {
		for {
			// This will be blocked by a channel read inside function
			if err := deviceHan.device.Start(deviceHan.stop); err != nil {
				logrus.Errorf("Error starting %s device plugin", deviceName)
			}

			select {
			case <-deviceHan.stop:
				return
			case <-time.After(5 * time.Second):
				// try to start device plugin again when getting error
				continue
			}
		}
	}()

	deviceHan.started = true
}

func (h *DevClaimHandler) stopDevicePlugin(deviceHan *deviceController) {
	if !deviceHan.started {
		return
	}

	close(deviceHan.stop)
	deviceHan.started = false
}

func (h *DevClaimHandler) findDevicePlugin(pluginDevices map[string][]*deviceplugins.PluginDevices, usbDevice *v1beta1.USBDevice) *deviceplugins.PluginDevices {
	var pluginDevice *deviceplugins.PluginDevices

	for resourceName, devices := range pluginDevices {
		for _, device := range devices {
			device := device
			for _, d := range device.Devices {
				logrus.Debugf("resourceName: %s, device: %v", resourceName, d)
				if usbDevice.Status.DevicePath == d.DevicePath {
					pluginDevice = device
					return pluginDevice
				}
			}
		}
	}

	return pluginDevice
}

func (h *DevClaimHandler) OnRemove(_ string, claim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if claim == nil || claim.DeletionTimestamp == nil {
		return claim, nil
	}

	usbDevice, err := h.usbDeviceCache.Get(claim.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("usbClient device not found")
			return claim, nil
		}
		return claim, err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	virt, err := h.virtClient.Get(KubeVirtNamespace, KubeVirtResource, metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	virtDp := virt.DeepCopy()

	if len(virtDp.Spec.Configuration.PermittedHostDevices.USB) == 0 {
		return claim, nil
	}

	usbs := virtDp.Spec.Configuration.PermittedHostDevices.USB

	// split target one if usb.ResourceName == usbDevice.Name

	for i, usb := range usbs {
		if usb.ResourceName == usbDevice.Status.ResourceName {
			usbs = append(usbs[:i], usbs[i+1:]...)
			break
		}
	}

	virtDp.Spec.Configuration.PermittedHostDevices.USB = usbs

	if !reflect.DeepEqual(virt.Spec.Configuration.PermittedHostDevices.USB, virtDp.Spec.Configuration.PermittedHostDevices.USB) {
		if _, err := h.virtClient.Update(virtDp); err != nil {
			return claim, nil
		}
	}

	if handler, ok := h.devicePlugin[claim.Name]; ok {
		h.stopDevicePlugin(handler)
		delete(h.devicePlugin, claim.Name)
	}

	usbDeviceCp := usbDevice.DeepCopy()
	usbDeviceCp.Status.Enabled = false
	if _, err = h.usbClient.UpdateStatus(usbDeviceCp); err != nil {
		logrus.Errorf("failed to disable usb device %s status: %v", usbDeviceCp.Name, err)
		return claim, err
	}

	return claim, nil
}

func (h *DevClaimHandler) updateKubeVirt(virt *kubevirtv1.KubeVirt, usbDevice *v1beta1.USBDevice) (*kubevirtv1.KubeVirt, error) {
	virtDp := virt.DeepCopy()

	if virtDp.Spec.Configuration.PermittedHostDevices == nil {
		virtDp.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			USB: make([]kubevirtv1.USBHostDevice, 0),
		}
	}

	usbs := virtDp.Spec.Configuration.PermittedHostDevices.USB

	// check if the usb device is already added
	for _, usb := range usbs {
		// skip same resource name
		if usb.ResourceName == usbDevice.Status.ResourceName {
			return virt, nil
		}
	}

	virtDp.Spec.Configuration.PermittedHostDevices.USB = append(usbs, kubevirtv1.USBHostDevice{
		Selectors: []kubevirtv1.USBSelector{
			{
				Vendor:  usbDevice.Status.VendorID,
				Product: usbDevice.Status.ProductID,
			},
		},
		ResourceName:             usbDevice.Status.ResourceName,
		ExternalResourceProvider: true,
	})

	if virt.Spec.Configuration.PermittedHostDevices == nil || !reflect.DeepEqual(virt.Spec.Configuration.PermittedHostDevices.USB, virtDp.Spec.Configuration.PermittedHostDevices.USB) {
		newVirt, err := h.virtClient.Update(virtDp)
		if err != nil {
			logrus.Errorf("failed to update kubevirt: %v", err)
			return virt, err
		}

		return newVirt, nil
	}

	return virt, nil
}

func convertToKubeVirtUSBFormat(ub *v1beta1.USBDevice) []kubevirtv1.USBHostDevice {
	return []kubevirtv1.USBHostDevice{
		{
			Selectors: []kubevirtv1.USBSelector{
				{
					Vendor:  ub.Status.VendorID,
					Product: ub.Status.ProductID,
				},
			},
			ResourceName:             ub.Status.ResourceName,
			ExternalResourceProvider: true,
		},
	}
}
