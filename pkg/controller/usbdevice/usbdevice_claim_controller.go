package usbdevice

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctlpcidevicerv1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

type ClaimHandler struct {
	usbClaimClient ctlpcidevicerv1.USBDeviceClaimController
	virtClient     kubecli.KubevirtClient
	lock           *sync.Mutex
	usbDeviceCache ctlpcidevicerv1.USBDeviceCache
	devicePlugin   map[string]*deviceplugins.USBDevicePlugin
}

func (h *ClaimHandler) OnUSBDeviceClaimChanged(_ string, usbDeviceClaim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if usbDeviceClaim == nil {
		return usbDeviceClaim, nil
	}

	usbDevice, err := h.usbDeviceCache.Get(usbDeviceClaim.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("usb device not found")
			return usbDeviceClaim, nil
		}
		return usbDeviceClaim, err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	virt, err := h.virtClient.KubeVirt("harvester-system").Get("kubevirt", &metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	newVirt, err := h.updateKubeVirt(virt, usbDevice)
	if err != nil {
		logrus.Errorf("failed to update kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	// start device plugin if kubevirt is updated.
	if _, ok := h.devicePlugin[usbDeviceClaim.Name]; !ok && newVirt != nil {
		pluginDevices := deviceplugins.DiscoverAllowedUSBDevices(newVirt.Spec.Configuration.PermittedHostDevices.USB)
		// same usbClient could have two more device with same resource name

		fmt.Println("part3")
		fmt.Println(len(pluginDevices))
		var pluginDevice *deviceplugins.PluginDevices

		for resourceName, devices := range pluginDevices {
			fmt.Println(resourceName)
			fmt.Println(len(devices))
			for _, device := range devices {
				fmt.Println("usbDevice.Status.DevicePath: ", usbDevice.Status.DevicePath)
				fmt.Println("device.Devices[0].DevicePath: ", device.Devices[0].DevicePath)
				if usbDevice.Status.DevicePath == device.Devices[0].DevicePath {
					pluginDevice = device
					break
				}
			}
		}

		fmt.Println("part4")
		if pluginDevice != nil {
			fmt.Println("Start device plugin: ", usbDevice.Name)
			usbDevicePlugin := deviceplugins.NewUSBDevicePlugin(usbDevice.Status.ResourceName, []*deviceplugins.PluginDevices{pluginDevice})
			h.devicePlugin[usbDeviceClaim.Name] = usbDevicePlugin
			go func() {
				fmt.Println("part6")
				sp := make(chan struct{})
				if err := usbDevicePlugin.Start(sp); err != nil {
					logrus.Errorf("failed to start device plugin: %v", err)
				}
				<-sp
			}()
		}
		fmt.Println("part5")
	}

	return usbDeviceClaim, nil
}

func (h *ClaimHandler) OnRemove(_ string, claim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if claim == nil {
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

	virt, err := h.virtClient.KubeVirt("harvester-system").Get("kubevirt", &metav1.GetOptions{})
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
		if _, err := h.virtClient.KubeVirt("harvester-system").Update(virtDp); err != nil {
			return claim, nil
		}
	}

	if dp, ok := h.devicePlugin[claim.Name]; ok {
		if err := dp.StopDevicePlugin(); err != nil {
			return claim, err
		}

		delete(h.devicePlugin, claim.Name)
	}

	return claim, nil
}

func (h *ClaimHandler) updateKubeVirt(virt *kubevirtv1.KubeVirt, usbDevice *v1beta1.USBDevice) (*kubevirtv1.KubeVirt, error) {
	virtDp := virt.DeepCopy()

	if virtDp.Spec.Configuration.PermittedHostDevices == nil {
		virtDp.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{}
	}

	if virtDp.Spec.Configuration.PermittedHostDevices.USB == nil {
		virtDp.Spec.Configuration.PermittedHostDevices.USB = make([]kubevirtv1.USBHostDevice, 0)
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
		newVirt, err := h.virtClient.KubeVirt("harvester-system").Update(virtDp)
		if err != nil {
			logrus.Errorf("failed to update kubevirt: %v", err)
			return virt, err
		}

		return newVirt, nil
	}

	return virt, nil
}
