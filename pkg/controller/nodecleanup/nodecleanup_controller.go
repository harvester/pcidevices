package nodecleanup

import (
	"context"
	"fmt"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/config"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	wranglerFinalizer = "wrangler.cattle.io/PCIDeviceClaimOnRemove"
)

type Handler struct {
	pdcClient                 v1beta1.PCIDeviceClaimClient
	pdClient                  v1beta1.PCIDeviceClient
	sriovNetworkDevicesClient v1beta1.SRIOVNetworkDeviceClient
	sriovGPUDevicesClient     v1beta1.SRIOVGPUDeviceClient
	vgpuDevicesClient         v1beta1.VGPUDeviceClient
	usbDeviceClaimClient      v1beta1.USBDeviceClaimClient
	usbDevicesClient          v1beta1.USBDeviceClient
	nodeDevicesClient         v1beta1.NodeClient

	nodeClient corecontrollers.NodeController
}

func (h *Handler) OnRemove(_ string, node *v1.Node) (*v1.Node, error) {
	if node == nil || node.DeletionTimestamp == nil {
		return node, nil
	}

	cleanupFuncs := []func(*v1.Node) error{
		h.removeNodeObject,
		h.removePCIDeviceClaimsOnNode,
		h.removePCIDevicesOnNode,
		h.removeSRIOVNetworkDevicesOnNode,
		h.removeUSBDeviceClaimsOnNode,
		h.removeUSBDevicesOnNode,
		h.removeSRIOVGPUDevicesOnNode,
		h.removeVGPUDevicesOnNode,
	}

	for _, fn := range cleanupFuncs {
		if err := fn(node); err != nil {
			return node, err
		}
	}

	return node, nil
}

func Register(ctx context.Context, management *config.FactoryManager) error {
	pdcClient := management.DeviceFactory.Devices().V1beta1().PCIDeviceClaim()
	pdClient := management.DeviceFactory.Devices().V1beta1().PCIDevice()
	nodeClient := management.CoreFactory.Core().V1().Node()
	sriovNetworkDevicesClient := management.DeviceFactory.Devices().V1beta1().SRIOVNetworkDevice()
	sriovGPUDevicesClient := management.DeviceFactory.Devices().V1beta1().SRIOVGPUDevice()
	vgpuDevicesClient := management.DeviceFactory.Devices().V1beta1().VGPUDevice()
	usbDeviceClaimClient := management.DeviceFactory.Devices().V1beta1().USBDeviceClaim()
	usbDevicesClient := management.DeviceFactory.Devices().V1beta1().USBDevice()
	nodeDevicesClient := management.DeviceFactory.Devices().V1beta1().Node()

	handler := &Handler{
		pdcClient:                 pdcClient,
		pdClient:                  pdClient,
		nodeClient:                nodeClient,
		sriovNetworkDevicesClient: sriovNetworkDevicesClient,
		sriovGPUDevicesClient:     sriovGPUDevicesClient,
		vgpuDevicesClient:         vgpuDevicesClient,
		usbDeviceClaimClient:      usbDeviceClaimClient,
		usbDevicesClient:          usbDevicesClient,
		nodeDevicesClient:         nodeDevicesClient,
	}
	nodeClient.OnRemove(ctx, "node-remove", handler.OnRemove)
	return nil
}

func containsFinalizer(finalizers []string, finalizer string) bool {
	for _, v := range finalizers {
		if v == finalizer {
			return true
		}
	}
	return false
}

func removeFinalizer(finalizers []string, finalizer string) []string {
	for i, v := range finalizers {
		if v == finalizer {
			return append(finalizers[:i], finalizers[i+1:]...)
		}
	}
	return finalizers
}

func (h *Handler) removePCIDevicesOnNode(node *v1.Node) error {
	// Delete all of that Node's PCIDevices
	selector := fmt.Sprintf("nodename=%s", node.Name)
	pds, err := h.pdClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error getting pds: %s", err)
		return err
	}
	for _, pd := range pds.Items {
		err = h.pdClient.Delete(pd.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting pd: %s", err)
			return err
		}
	}
	return nil
}

func (h *Handler) removePCIDeviceClaimsOnNode(node *v1.Node) error {
	logrus.Debugf("cleaning pcidevices for node %s", node.Name)
	// Delete all of that Node's PCIDeviceClaims
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("error getting pdcs: %s", err)
		return err
	}
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName != node.Name {
			continue
		}
		pdcCopy := pdc.DeepCopy()
		if containsFinalizer(pdcCopy.Finalizers, wranglerFinalizer) {
			pdcCopy.Finalizers = removeFinalizer(pdcCopy.Finalizers, wranglerFinalizer)
			_, err := h.pdcClient.Update(pdcCopy)
			if err != nil {
				return fmt.Errorf("error removing finalizer: %v", err)
			}
		}

		err = h.pdcClient.Delete(pdc.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting pdc: %s", err)
			return err
		}
	}
	return nil
}

func (h *Handler) removeSRIOVNetworkDevicesOnNode(node *v1.Node) error {
	// Delete all SRIOVNetworkDevices related to the node
	selector := fmt.Sprintf("nodename=%s", node.Name)
	sriovNetworkDevices, err := h.sriovNetworkDevicesClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error listing sriovnetworkdevices for node %s: %v", node.Name, err)
		return err
	}
	for _, sriovNetworkDevice := range sriovNetworkDevices.Items {
		err = h.sriovNetworkDevicesClient.Delete(sriovNetworkDevice.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting sriovnetworkdevice: %v", err)
			return err
		}
	}
	return nil
}

func (h *Handler) removeUSBDeviceClaimsOnNode(node *v1.Node) error {
	// Delete all USBDeviceClaims related to the node
	usbDeviceClaims, err := h.usbDeviceClaimClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("error getting usbDeviceclaims: %v", err)
		return err
	}

	for _, usbDeviceClaim := range usbDeviceClaims.Items {
		// ignore devices from nodes which are not being deleted
		if usbDeviceClaim.Status.NodeName != node.Name {
			continue
		}
		// patch finalizers on the usbDeviceClaim
		if len(usbDeviceClaim.Finalizers) != 0 {
			usbDeviceClaim.SetFinalizers(nil)
			_, err := h.usbDeviceClaimClient.Update(&usbDeviceClaim)
			if err != nil {
				logrus.Errorf("error updating usbdeviceClaim %s to remove finalizers %v", usbDeviceClaim.Name, err)
				return err
			}
		}

		err = h.usbDeviceClaimClient.Delete(usbDeviceClaim.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting usbdeviceClaim %s: %v", usbDeviceClaim.Name, err)
			return err
		}
	}
	return nil
}

func (h *Handler) removeUSBDevicesOnNode(node *v1.Node) error {
	// Delete all USBDevices related to the node
	selector := fmt.Sprintf("nodename=%s", node.Name)
	usbDevices, err := h.usbDevicesClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error listing usbDevices for node %s: %v", node.Name, err)
		return err
	}

	for _, usbDevice := range usbDevices.Items {
		err = h.usbDevicesClient.Delete(usbDevice.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting device %s: %v", usbDevice.Name, err)
			return err
		}
	}

	return nil
}

func (h *Handler) removeSRIOVGPUDevicesOnNode(node *v1.Node) error {
	// Delete all SRIOVGPUDevices related to the node
	selector := fmt.Sprintf("nodename=%s", node.Name)
	sriovGPUDevices, err := h.sriovGPUDevicesClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error listing sriovGPUDevices for node %s: %v", node.Name, err)
		return err
	}
	for _, sriovGPUDevice := range sriovGPUDevices.Items {
		err = h.sriovGPUDevicesClient.Delete(sriovGPUDevice.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting sriovGPUDevice %s: %v", sriovGPUDevice.Name, err)
			return err
		}
	}
	return nil
}

func (h *Handler) removeVGPUDevicesOnNode(node *v1.Node) error {
	selector := fmt.Sprintf("nodename=%s", node.Name)
	vgpuDevices, err := h.vgpuDevicesClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error listing vgpuDevices for node %s: %v", node.Name, err)
		return err
	}

	for _, vgpuDevice := range vgpuDevices.Items {
		err = h.vgpuDevicesClient.Delete(vgpuDevice.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting vgpuDevice %s: %v", vgpuDevice.Name, err)
			return err
		}
	}

	return nil
}

func (h *Handler) removeNodeObject(node *v1.Node) error {
	// delete the node.devices object used to reconcile /sys fs objects
	err := h.nodeDevicesClient.Delete(node.Name, &metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("error deleting node.devices %s: %v", node.Name, err)
			return err
		}
	}
	return nil
}
