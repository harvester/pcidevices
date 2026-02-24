package gpudevice

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

const (
	DefaultNS   = "harvester-system"
	KubevirtCR  = "kubevirt"
	defaultUser = "admin"
)

func (h *Handler) OnVGPUChange(_ string, vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	discoveredVGPUStatus, err := gpuhelper.FetchVGPUStatus(v1beta1.SysDevRoot, vgpu.Spec.Address)
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
	// perform out of band enable / disable operation if needed //
	// with switch to vendor specific nvidia driver implementation in Harvester v1.8.0
	// it is not possible to fetch configured VGPUTypeName from sysfs, only id, which is
	// available in UUID, so we manually copy and check if an update is needed
	vgpuCopy := vgpu.DeepCopy()
	vgpuCopy.Status.UUID = discoveredVGPUStatus.UUID
	vgpuCopy.Status.VGPUStatus = discoveredVGPUStatus.VGPUStatus
	vgpuCopy.Status.AvailableTypes = discoveredVGPUStatus.AvailableTypes
	if !reflect.DeepEqual(vgpuCopy.Status, vgpu.Status) {
		return h.vGPUClient.UpdateStatus(vgpuCopy)
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
	vgpuID, ok := vgpu.Status.AvailableTypes[vgpu.Spec.VGPUTypeName]
	if !ok {
		return vgpu, fmt.Errorf("VGPUType specified %s is not available for vGPU %s", vgpu.Spec.VGPUTypeName, vgpu.Spec.Address)
	}

	vgpuUUID := vgpuID

	// setup pcidevice claims / pcidevice objects
	if err := h.submitPCIDeviceClaim(vgpu); err != nil && !apierrors.IsAlreadyExists(err) {
		return vgpu, fmt.Errorf("error creating pcideviceclaim for associated vgpu device: %w", err)
	}

	// setup vgpu profile
	createFilePath := filepath.Join(v1beta1.SysDevRoot, vgpu.Spec.Address, "nvidia", v1beta1.CurrentVGPUType)

	if err := os.WriteFile(createFilePath, []byte(vgpuID), 0600); err != nil {
		return vgpu, fmt.Errorf("error writing to create file for vgpu %s: %v", vgpu.Name, err)
	}

	vgpu.Status.VGPUStatus = v1beta1.VGPUEnabled
	vgpu.Status.UUID = vgpuUUID
	vgpu.Status.ConfiguredVGPUTypeName = vgpu.Spec.VGPUTypeName
	vgpuObj, err := h.vGPUClient.UpdateStatus(vgpu)
	if err != nil {
		return vgpuObj, err
	}
	// need to create pcidevice claim for pcidevice associated with vgpu device
	return h.reconcileDisabledVGPUStatus(vgpuObj)
}

// disableVGPU performs the op to disable VGPU
func (h *Handler) disableVGPU(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	// cleanup pcidevice claim
	if err := h.cleanupRelatedPCIDeviceObjects(vgpu); err != nil {
		return vgpu, err
	}

	createFilePath := filepath.Join(v1beta1.SysDevRoot, vgpu.Spec.Address, "nvidia", v1beta1.CurrentVGPUType)

	if err := os.WriteFile(createFilePath, []byte("0"), 0600); err != nil {
		return vgpu, fmt.Errorf("error writing to create file for vgpu %s: %v", vgpu.Name, err)
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

func (h *Handler) submitPCIDeviceClaim(vgpu *v1beta1.VGPUDevice) error {

	resourceName := gpuhelper.GenerateDeviceName(vgpu.Spec.VGPUTypeName)

	logrus.Debugf("sending resource name %s for vgpu %s", resourceName, vgpu.Name)
	pcidevice, err := h.pciDeviceCache.Get(vgpu.Name)
	if err != nil {
		return fmt.Errorf("error looking up pcidevice when trying to setup vgpu passthrough: %w", err)
	}

	// patch pcidevice resource name in status
	if err := h.patchPCIDeviceStatus(vgpu.Name, resourceName); err != nil {
		return fmt.Errorf("error patching pcidevice status resource name for vgpu %s: %w", vgpu.Name, err)
	}

	// patch pcidevice resource name in spec
	// this is needed to ensure that regularly scheduled pcidevice reconcile
	// does not wipe the resource name causing issues with plugin
	if err := h.patchPCIDeviceSpec(vgpu.Name, resourceName); err != nil {
		return fmt.Errorf("error patching pcidevice resource name for vgpu %s: %w", vgpu.Name, err)
	}

	pcideviceClaim := &v1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: vgpu.Name,
			Labels: map[string]string{
				v1beta1.ParentSRIOVGPUDeviceLabel: v1beta1.PCIDeviceNameForHostname(vgpu.Spec.ParentGPUDeviceAddress, vgpu.Spec.NodeName),
			},
			Annotations: map[string]string{
				v1beta1.SkipVFIOBindingAnnotationKey:  "true",
				v1beta1.PCIDeviceOverrideResourceName: resourceName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       pcidevice.Kind,
					Name:       pcidevice.Name,
					UID:        pcidevice.UID,
					APIVersion: pcidevice.APIVersion,
				},
				{
					Kind:       vgpu.Kind,
					Name:       vgpu.Name,
					UID:        vgpu.UID,
					APIVersion: vgpu.APIVersion,
				},
			},
		},
		Spec: v1beta1.PCIDeviceClaimSpec{
			Address:  vgpu.Spec.Address,
			UserName: defaultUser,
			NodeName: vgpu.Spec.NodeName,
		},
	}

	logrus.Debugf("submitting pcidevice claim for vgpu %s", vgpu.Name)
	_, err = h.pciDeviceClaim.Create(pcideviceClaim)
	return err
}

func (h *Handler) cleanupRelatedPCIDeviceObjects(vgpu *v1beta1.VGPUDevice) error {
	err := h.pciDeviceClaim.Delete(vgpu.Name, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return h.cleanupPCIDeviceSpec(vgpu.Name)
}

func (h *Handler) patchPCIDeviceSpec(vgpu, resourceName string) error {
	pdObj, err := h.pciDeviceCache.Get(vgpu)
	if err != nil {
		return fmt.Errorf("error looking up pcidevice %s in patchPCIDeviceSpec: %w", vgpu, err)
	}

	if pdObj.Annotations == nil {
		pdObj.Annotations = make(map[string]string)
	}
	pdObjCopy := pdObj.DeepCopy()
	pdObjCopy.Annotations[v1beta1.PCIDeviceOverrideResourceName] = resourceName
	// need to update pcidevice
	if reflect.DeepEqual(pdObj, pdObjCopy) {
		return nil
	}
	_, err = h.pciDevice.Update(pdObjCopy)
	return err
}

func (h *Handler) patchPCIDeviceStatus(vgpu, resourceName string) error {
	pdObj, err := h.pciDeviceCache.Get(vgpu)
	if err != nil {
		return fmt.Errorf("error looking up pcidevice %s in patchPCIDeviceStatus: %w", vgpu, err)
	}
	pdObjCopy := pdObj.DeepCopy()
	pdObjCopy.Status.ResourceName = resourceName
	if reflect.DeepEqual(pdObj.Status, pdObjCopy.Status) {
		return nil
	}
	logrus.Debugf("updating pd status for vgpu %s: %v", vgpu, pdObjCopy.Status)
	_, err = h.pciDevice.UpdateStatus(pdObjCopy)
	return err
}

func (h *Handler) cleanupPCIDeviceSpec(vgpu string) error {
	pdObj, err := h.pciDeviceCache.Get(vgpu)
	if err != nil {
		return fmt.Errorf("error looking up pcidevice %s in cleanupPCIDeviceSpec: %w", vgpu, err)
	}
	pdObjCopy := pdObj.DeepCopy()
	// we just remove the annotation, and regular reconcile of pcidevice will eventually update
	// resource name in status as well
	delete(pdObjCopy.Annotations, v1beta1.PCIDeviceOverrideResourceName)
	// need to update pcidevice
	if reflect.DeepEqual(pdObj, pdObjCopy) {
		return nil
	}
	_, err = h.pciDevice.Update(pdObjCopy)
	return err
}
