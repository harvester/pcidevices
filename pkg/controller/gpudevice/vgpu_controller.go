package gpudevice

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

var (
	pluginLock sync.Mutex
)

const (
	DefaultNS  = "harvester-system"
	KubevirtCR = "kubevirt"
)

func (h *Handler) OnVGPUChange(_ string, vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	discoveredVGPUStatus, err := gpuhelper.FetchVGPUStatus(v1beta1.MdevRoot, v1beta1.SysDevRoot, v1beta1.MdevBusClassRoot, vgpu.Spec.Address)
	if err != nil {
		return vgpu, fmt.Errorf("error generating vgpu %s status: %v", vgpu.Name, err)
	}

	// gpu spec is enabled and discovered status indicates no configuration
	if vgpu.Spec.Enabled && discoveredVGPUStatus.VGPUStatus == v1beta1.VGPUDisabled {
		return h.enableVGPU(vgpu)
	}

	if !vgpu.Spec.Enabled && discoveredVGPUStatus.VGPUStatus == v1beta1.VGPUEnabled {
		return h.disableVGPU(vgpu)
	}
	// perform enable disable operation //
	if !reflect.DeepEqual(discoveredVGPUStatus, vgpu.Status) {
		vgpu.Status = *discoveredVGPUStatus
		return h.vGPUClient.UpdateStatus(vgpu)
	}

	return nil, nil
}

func (h *Handler) SetupVGPUDevices() error {
	vGPUDevices, err := gpuhelper.IdentifyVGPU(h.options, h.nodeName)
	if err != nil {
		return fmt.Errorf("error identifying vgpu devices: %v", err)
	}
	return h.reconcileVGPUSetup(vGPUDevices)
}

func (h *Handler) reconcileVGPUSetup(vGPUDevices []*v1beta1.VGPUDevice) error {
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}

	vGPUList, err := h.vGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return err
	}

	for _, v := range vGPUDevices {
		existingVGPU := containsVGPU(v, vGPUList)
		if existingVGPU != nil {
			if !reflect.DeepEqual(v.Status, existingVGPU.Status) {
				// on reboot the vGPU status will not match the state in CRD
				// in which case we should if needed reset the vGPU status and
				// allow reconcile to flow through
				existingVGPU.Status = v.Status
				if _, err := h.vGPUClient.UpdateStatus(existingVGPU); err != nil {
					return err
				}
			}
		} else {
			if _, err := h.vGPUClient.Create(v); err != nil {
				return err
			}
		}
	}

	for _, v := range vGPUList {
		parentDeviceEnabled, err := h.isParentGPUEnabled(v.Spec.ParentGPUDeviceAddress)
		if err != nil {
			return err
		}

		// if parentDevice is enabled, then skip deletion
		// as controller will reconcile and re-configure vGPU when device parent SRIOVGPU device
		// is reconfigured post reboot
		if vGPUExists := containsVGPU(v, vGPUDevices); !parentDeviceEnabled && vGPUExists == nil {
			if err := h.vGPUClient.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsVGPU(vgpu *v1beta1.VGPUDevice, vgpuList []*v1beta1.VGPUDevice) *v1beta1.VGPUDevice {
	for _, v := range vgpuList {
		if vgpu.Name == v.Name {
			return v
		}
	}
	return nil
}

// enableVGPU performs the op to configure VGPU
func (h *Handler) enableVGPU(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	nvidiaType, ok := vgpu.Status.AvailableTypes[vgpu.Spec.VGPUTypeName]
	if !ok {
		return vgpu, fmt.Errorf("VGPUType specified %s is not available for vGPU %s", vgpu.Spec.VGPUTypeName, vgpu.Spec.Address)
	}

	vgpuUUID := uuid.NewString()

	createFilePath := filepath.Join(v1beta1.MdevBusClassRoot, vgpu.Spec.Address, v1beta1.MdevSupportTypesDir, nvidiaType, "create")
	if _, err := os.Stat(createFilePath); err != nil {
		return vgpu, fmt.Errorf("error looking up create file for vgpu %s: %v", vgpu.Name, err)
	}

	if err := os.WriteFile(createFilePath, []byte(vgpuUUID), fs.FileMode(os.O_WRONLY)); err != nil {
		return vgpu, fmt.Errorf("error writing to create file for vgpu %s: %v", vgpu.Name, err)
	}

	vgpu.Status.VGPUStatus = v1beta1.VGPUEnabled
	vgpu.Status.UUID = vgpuUUID
	vgpuObj, err := h.vGPUClient.UpdateStatus(vgpu)
	if err != nil {
		return vgpuObj, err
	}

	return h.reconcileDisabledVGPUStatus(vgpuObj)
}

// disableVGPU performs the op to disable VGPU
func (h *Handler) disableVGPU(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	removeFile := filepath.Join(v1beta1.MdevBusClassRoot, vgpu.Spec.Address, vgpu.Status.UUID, "remove")
	found := true
	// possible that CRD update fails but file has been removed
	// this can lead to issue during reconcile.
	// in such a case we just ensure plugin is updated and CRD status reflects disabled state
	if _, err := os.Stat(removeFile); err != nil {
		if os.IsNotExist(err) {
			found = false
		} else {
			return vgpu, fmt.Errorf("error looking up remove file for vgpu %s: %v", vgpu.Name, err)
		}
	}

	if found {
		if err := os.WriteFile(removeFile, []byte("1"), fs.FileMode(os.O_WRONLY)); err != nil {
			return vgpu, fmt.Errorf("error writing to remove file for vgpu %s: %v", vgpu.Name, err)
		}
	}

	// disableDevicePlugin is run here as we need UUID to remove the device
	if err := h.disableDevicePlugin(vgpu); err != nil {
		return vgpu, fmt.Errorf("error cleaning up device plugin for device %s: %v", vgpu.Name, err)
	}

	vgpu.Status.VGPUStatus = v1beta1.VGPUDisabled
	vgpu.Status.UUID = ""
	vgpu.Status.ConfiguredVGPUTypeName = ""
	vgpuObj, err := h.vGPUClient.UpdateStatus(vgpu)
	if err != nil {
		return vgpuObj, err
	}

	return h.reconcileDisabledVGPUStatus(vgpuObj)
}

func (h *Handler) disableDevicePlugin(vgpu *v1beta1.VGPUDevice) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	pluginName := gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName)
	plugin, ok := h.vGPUDevicePlugins[pluginName]
	if !ok {
		logrus.Debugf("no device plugin found for vgpu %s of type %s", vgpu.Name, vgpu.Status.ConfiguredVGPUTypeName)
		return nil
	}

	if err := plugin.RemoveDevice(vgpu.Status.UUID); err != nil {
		return fmt.Errorf("error removing device: %v", err)
	}

	if plugin.GetCount() == 0 {
		logrus.Infof("shutting down device plugin for %s", pluginName)
		if err := plugin.Stop(); err != nil {
			return err
		}
		delete(h.vGPUDevicePlugins, pluginName)
	}
	return nil
}

// reconcileEnabledVGPUPlugins runs as an out of band handler from the VGPU Device management loop. This is needed as we reconcile CRD to OS state.
// in case there was an error during CRD status update, subsequent reconcile will generate correct status from CRD.
// the enable subroutine is skipped in this case and placing the device plugin enable logic will likely miss some devices
func (h *Handler) reconcileEnabledVGPUPlugins(_ string, vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	// post reboot the vgpu devices are cleared from /sys
	// as a result we need to wait until the device exists in /sys/bus/mdev/devices
	// else fake devices get added to the node

	discoveredVGPUStatus, err := gpuhelper.FetchVGPUStatus(v1beta1.MdevRoot, v1beta1.SysDevRoot, v1beta1.MdevBusClassRoot, vgpu.Spec.Address)
	if err != nil {
		return vgpu, err
	}
	if vgpu.Spec.Enabled && discoveredVGPUStatus.UUID != "" && discoveredVGPUStatus.ConfiguredVGPUTypeName != "" {
		vgpuCopy := vgpu.DeepCopy()
		vgpuCopy.Status.ConfiguredVGPUTypeName = discoveredVGPUStatus.ConfiguredVGPUTypeName
		return vgpu, h.createOrUpdateDevicePlugin(vgpuCopy)
	}

	return vgpu, nil
}

func (h *Handler) createOrUpdateDevicePlugin(vgpu *v1beta1.VGPUDevice) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()

	if err := h.whiteListVGPU(vgpu); err != nil {
		return err
	}

	pluginName := gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName)
	plugin, ok := h.vGPUDevicePlugins[pluginName]
	if ok {
		// plugin exists. just publish address and move on
		if !plugin.DeviceExists(vgpu.Status.UUID) {
			return plugin.AddDevice(vgpu.Status.UUID)
		}
		return nil
	}

	newPlugin := deviceplugins.NewVGPUDevicePlugin(h.ctx, []string{vgpu.Status.UUID}, pluginName)
	if err := h.startDevicePlugin(newPlugin); err != nil {
		return err
	}

	h.vGPUDevicePlugins[pluginName] = newPlugin
	return nil
}

func (h *Handler) startDevicePlugin(
	dp *deviceplugins.VGPUDevicePlugin,
) error {
	if dp.Started() {
		return nil
	}
	// Start the plugin
	stop := make(chan struct{})
	go func() {
		err := dp.Start(stop)
		if err != nil {
			logrus.Errorf("error starting %s device plugin: %s", dp.GetDeviceName(), err)
		}
		// TODO: test if deleting this stops the DevicePlugin
		<-stop
	}()
	dp.SetStarted(stop)
	return nil
}

// whiteListGPU checks if VGPU type is already whitelisted in the kubevirt CR.
// if not it does the whitelisting
func (h *Handler) whiteListVGPU(vgpu *v1beta1.VGPUDevice) error {
	kv, err := h.virtClient.KubeVirt(DefaultNS).Get(KubevirtCR, &metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error looking up kubevirt CR: %v", err)
	}

	if kv.Spec.Configuration.PermittedHostDevices == nil {
		kv.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{}
	}

	for _, v := range kv.Spec.Configuration.PermittedHostDevices.MediatedDevices {
		if v.ResourceName == gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName) && v.MDEVNameSelector == vgpu.Status.ConfiguredVGPUTypeName && v.ExternalResourceProvider {
			logrus.Debugf("device type %s already whitelisted, no further action needed", vgpu.Status.ConfiguredVGPUTypeName)
			return nil
		}
	}

	kv.Spec.Configuration.PermittedHostDevices.MediatedDevices = append(kv.Spec.Configuration.PermittedHostDevices.MediatedDevices, kubevirtv1.MediatedHostDevice{
		ResourceName:             gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName),
		MDEVNameSelector:         vgpu.Status.ConfiguredVGPUTypeName,
		ExternalResourceProvider: true,
	})

	_, err = h.virtClient.KubeVirt(DefaultNS).Update(kv)
	return err
}

// reconcileDisabledVGPUStatus is needed as when a vgpu is enabled, based on type of vGPU the available types for other vgpu's may change. This ensures that the state of other vGPU's from the same parent GPU is reconciled immediately to avoid users from attempting to enable unsupported vGPU Types
func (h *Handler) reconcileDisabledVGPUStatus(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	// if a vgpu has been recently configured, then find all related and disabled VGPU's to reconcile availableGPUTypes in status
	set := map[string]string{
		v1beta1.ParentSRIOVGPUDeviceLabel: v1beta1.PCIDeviceNameForHostname(vgpu.Spec.ParentGPUDeviceAddress, h.nodeName),
	}
	vgpuList, err := h.vGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return vgpu, fmt.Errorf("error querying related vgpu's for vgpu %s: %v", vgpu.Name, err)
	}

	for _, v := range vgpuList {
		if v.Spec.Enabled || v.Name == vgpu.Name {
			continue
		}
		// enqueue vGPU's to trigger reconcile of new status
		logrus.Debugf("requeue device %s to force status reconcile", v.Name)
		h.vGPUController.Enqueue(v.Name)
	}
	return vgpu, nil
}

// isParentGPUEnabled checks if parentGPU is needed. This is need during reconcile of vGPU's post a reboot
// as it can take a while for GPU to be re-enabled post reboot, while node reconcile could end up deleting
// vGPU devices. This avoids deletion of said vGPU devices during node object reconcile
func (h *Handler) isParentGPUEnabled(gpuAddress string) (bool, error) {
	parentSRIOVGPUDeviceName := v1beta1.PCIDeviceNameForHostname(gpuAddress, h.nodeName)
	parentSRIOVGPUDevice, err := h.sriovGPUCache.Get(parentSRIOVGPUDeviceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return parentSRIOVGPUDevice.Spec.Enabled, nil
}
