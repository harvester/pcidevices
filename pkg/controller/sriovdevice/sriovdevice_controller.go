package sriovdevice

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/jaypipes/ghw"
	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/config"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/nichelper"
)

type Handler struct {
	ctx             context.Context
	sriovCache      ctl.SRIOVNetworkDeviceCache
	sriovClient     ctl.SRIOVNetworkDeviceClient
	nodeName        string
	nodeCache       ctlcorev1.NodeCache
	vlanConfigCache ctlnetworkv1beta1.VlanConfigCache
}

const (
	reconcileSriovDevice = "reconcile-sriovdevice"
)

func NewHandler(ctx context.Context, sriovCache ctl.SRIOVNetworkDeviceCache, sriovClient ctl.SRIOVNetworkDeviceClient, nodeName string,
	nodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache) *Handler {
	return &Handler{
		ctx:             ctx,
		sriovCache:      sriovCache,
		sriovClient:     sriovClient,
		nodeName:        nodeName,
		nodeCache:       nodeCache,
		vlanConfigCache: vlanConfigCache,
	}
}

func Register(ctx context.Context, management *config.FactoryManager) error {
	sriovDeviceController := management.DeviceFactory.Devices().V1beta1().SRIOVNetworkDevice()
	nodeCache := management.CoreFactory.Core().V1().Node().Cache()
	vlanConfigCache := management.NetworkFactory.Network().V1beta1().VlanConfig().Cache()
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)

	h := NewHandler(ctx, sriovDeviceController.Cache(), sriovDeviceController, nodeName,
		nodeCache, vlanConfigCache)
	sriovDeviceController.OnChange(ctx, reconcileSriovDevice, h.reconcileSriovDevice)
	return nil
}

func (h *Handler) SetupSriovDevices() error {
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}

	sriovDeviceList, err := h.sriovCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return fmt.Errorf("error listing sriovdevices for node %s: %v", h.nodeName, err)
	}

	nics, err := ghw.Network()
	if err != nil {
		return fmt.Errorf("error listing nics for node %s: %v", h.nodeName, err)
	}

	generatedSrioDeviceList, err := nichelper.GenerateSRIOVNics(h.nodeName, h.nodeCache, h.vlanConfigCache, nics)
	if err != nil {
		return fmt.Errorf("error generating sriovdevice list for node %s: %v", h.nodeName, err)
	}

	// create sriov devices if no present
	for _, dev := range generatedSrioDeviceList {
		if !containsSRIOVDevice(dev, sriovDeviceList) {
			_, err = h.sriovClient.Create(dev)
			if err != nil {
				return fmt.Errorf("error creating device %s on node %s: %v", dev.Name, h.nodeName, err)
			}
		}
	}

	// remove sriovdevice objects no longer present
	for _, dev := range sriovDeviceList {
		if !containsSRIOVDevice(dev, generatedSrioDeviceList) {
			err = h.sriovClient.Delete(dev.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("error deleting sriov device %s on node %s: %v", dev.Name, h.nodeName, err)
			}
		}
	}

	return nil
}

func containsSRIOVDevice(key *v1beta1.SRIOVNetworkDevice, values []*v1beta1.SRIOVNetworkDevice) bool {
	for _, v := range values {
		if v.Name == key.Name {
			return true
		}
	}
	return false
}

func (h *Handler) reconcileSriovDevice(_ string, sriovDevice *v1beta1.SRIOVNetworkDevice) (*v1beta1.SRIOVNetworkDevice, error) {
	if sriovDevice == nil || sriovDevice.DeletionTimestamp != nil || sriovDevice.Spec.NodeName != h.nodeName {
		return sriovDevice, nil
	}

	// if device is not enabled, ensure numvfs is disabled on underlying device
	if sriovDevice.Spec.NumVFs == 0 {
		return h.ensureDeviceIsDisabled(sriovDevice)
	}

	return h.ensureDeviceIsConfigured(sriovDevice)
}

func (h *Handler) ensureDeviceIsDisabled(sriovDevice *v1beta1.SRIOVNetworkDevice) (*v1beta1.SRIOVNetworkDevice, error) {
	deviceCopy := sriovDevice.DeepCopy()

	vfs, err := nichelper.CurrentVFConfigured(sriovDevice.Spec.Address)
	if err != nil {
		return sriovDevice, fmt.Errorf("error querying number of vf's on device %s: %v", sriovDevice.Name, err)
	}

	if vfs != 0 {
		if err := nichelper.ConfigureVF(sriovDevice.Spec.Address, 0); err != nil {
			return sriovDevice, fmt.Errorf("error setting vf count to 0 on device %s: %v", sriovDevice.Name, err)
		}
		deviceCopy.Status.Status = v1beta1.DeviceDisabled
		deviceCopy.Status.VFAddresses = nil
	}

	if !reflect.DeepEqual(deviceCopy.Status, sriovDevice.Status) {
		return h.sriovClient.UpdateStatus(deviceCopy)
	}

	return sriovDevice, nil
}

func (h *Handler) ensureDeviceIsConfigured(sriovDevice *v1beta1.SRIOVNetworkDevice) (*v1beta1.SRIOVNetworkDevice, error) {
	deviceCopy := sriovDevice.DeepCopy()

	vfs, err := nichelper.CurrentVFConfigured(sriovDevice.Spec.Address)
	if err != nil {
		return sriovDevice, fmt.Errorf("error querying number of vf's on device %s: %v", sriovDevice.Name, err)
	}

	if vfs != deviceCopy.Spec.NumVFs {
		if err := nichelper.ConfigureVF(sriovDevice.Spec.Address, sriovDevice.Spec.NumVFs); err != nil {
			return sriovDevice, fmt.Errorf("error setting vf count to %d on device %s: %v", sriovDevice.Spec.NumVFs, sriovDevice.Name, err)
		}
	}

	vfAddresses, err := nichelper.GetVFList(deviceCopy.Spec.Address)
	if err != nil {
		return sriovDevice, fmt.Errorf("error looking up vf addresses for device %s: %v", deviceCopy.Spec.Address, err)
	}

	vfPCIDevices := make([]string, 0, len(vfAddresses))
	for _, vf := range vfAddresses {
		vfPCIDevices = append(vfPCIDevices, v1beta1.PCIDeviceNameForHostname(vf, h.nodeName))
	}
	deviceCopy.Status.Status = v1beta1.DeviceEnabled
	deviceCopy.Status.VFPCIDevices = vfPCIDevices
	deviceCopy.Status.VFAddresses = vfAddresses

	if !reflect.DeepEqual(deviceCopy.Status, sriovDevice.Status) {
		return h.sriovClient.UpdateStatus(deviceCopy)
	}

	return sriovDevice, nil
}
