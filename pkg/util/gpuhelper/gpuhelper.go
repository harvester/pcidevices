package gpuhelper

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
)

const (
	creatableVGPUTypes = "creatable_vgpu_types"
	currentVGPUType    = "current_vgpu_type"
)

func IdentifySRIOVGPU(options []nvpci.Option, hostname string) ([]*v1beta1.SRIOVGPUDevice, error) {
	mgr := nvpci.New(options...)
	sriovGPUDevices := make([]*v1beta1.SRIOVGPUDevice, 0)
	nvidiaGPU, err := mgr.GetGPUs()
	if err != nil {
		return nil, fmt.Errorf("error querying GPU's: %v", err)
	}

	for _, v := range nvidiaGPU {
		ok, err := common.IsDeviceSRIOVCapable(v.Path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		devObj := generateSRIOVGPUDevice(v, hostname)
		enabled, devObjStatus, err := GenerateGPUStatus(v.Path, hostname)
		if err != nil {
			return nil, err
		}
		devObj.Spec.Enabled = enabled
		devObj.Status = *devObjStatus
		logrus.Debugf("generated device: %v", *devObj)
		sriovGPUDevices = append(sriovGPUDevices, devObj)

	}

	return sriovGPUDevices, nil
}

func generateSRIOVGPUDevice(nvidiaGpu *nvpci.NvidiaPCIDevice, nodeName string) *v1beta1.SRIOVGPUDevice {
	obj := &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.PCIDeviceNameForHostname(nvidiaGpu.Address, nodeName),
			Labels: map[string]string{
				v1beta1.NodeKeyName: nodeName,
			},
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  nvidiaGpu.Address,
			NodeName: nodeName,
		},
	}
	return obj
}

func GenerateGPUStatus(devicePath string, hostname string) (bool, *v1beta1.SRIOVGPUDeviceStatus, error) {
	var enabled bool
	var err error
	count, err := common.CurrentVFConfigured(devicePath)
	if err != nil {
		return enabled, nil, err
	}

	if count > 0 {
		enabled = true
	}

	vfs, err := common.GetVFList(devicePath)
	if err != nil {
		return enabled, nil, err
	}

	deviceStatus := &v1beta1.SRIOVGPUDeviceStatus{}
	deviceStatus.VFAddresses = vfs
	for _, v := range vfs {
		deviceStatus.VGPUDevices = append(deviceStatus.VGPUDevices, v1beta1.PCIDeviceNameForHostname(v, hostname))
	}
	return enabled, deviceStatus, nil
}

func IdentifyVGPU(options []nvpci.Option, nodeName string) ([]*v1beta1.VGPUDevice, error) {
	vGPUDevices := make([]*v1beta1.VGPUDevice, 0)
	mgr := nvpci.New(options...)
	nvidiaDevices, err := mgr.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("error querying devices: %s", err)
	}

	for _, v := range nvidiaDevices {
		if v.IsGPU() && v.SriovInfo.IsVF() {
			dev, err := generateVGPUDevice(v, nodeName)
			if err != nil {
				return nil, err
			}
			vGPUDevices = append(vGPUDevices, dev)
		}
	}
	return vGPUDevices, nil
}

// generateVGPUDevice generates the v1beta1.VGPUDevice for corresponding NvidiaPCIDevice
func generateVGPUDevice(device *nvpci.NvidiaPCIDevice, nodeName string) (*v1beta1.VGPUDevice, error) {
	physFn, err := evalPhysFn(device.Path)
	if err != nil {
		return nil, err
	}

	vgpu := &v1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.PCIDeviceNameForHostname(device.Address, nodeName),
			Labels: map[string]string{
				v1beta1.NodeKeyName:               nodeName,
				v1beta1.ParentSRIOVGPUDeviceLabel: v1beta1.PCIDeviceNameForHostname(physFn, nodeName),
			},
		},
		Spec: v1beta1.VGPUDeviceSpec{
			Address:                device.Address,
			NodeName:               nodeName,
			ParentGPUDeviceAddress: physFn,
		},
	}

	status, err := FetchVGPUStatus(v1beta1.SysDevRoot, device.Address)
	if err != nil {
		return nil, err
	}
	if status.ConfiguredVGPUTypeName != "" {
		vgpu.Spec.Enabled = true
	}
	vgpu.Status = *status
	return vgpu, nil
}

/*
Read the contents of creatable_vgpu_types from the vGPU device
# cd /sys/bus/pci/devices/0000\:3d\:00.4/nvidia
# cat creatable_vgpu_types
ID    : vGPU Name
1145  : NVIDIA L40S-1B
1146  : NVIDIA L40S-2B
1147  : NVIDIA L40S-1Q
1148  : NVIDIA L40S-2Q
1149  : NVIDIA L40S-3Q
1150  : NVIDIA L40S-4Q
1151  : NVIDIA L40S-6Q
1152  : NVIDIA L40S-8Q
1153  : NVIDIA L40S-12Q
1154  : NVIDIA L40S-16Q
1155  : NVIDIA L40S-24Q
1156  : NVIDIA L40S-48Q
1157  : NVIDIA L40S-1A
1158  : NVIDIA L40S-2A
1159  : NVIDIA L40S-3A
1160  : NVIDIA L40S-4A
1161  : NVIDIA L40S-6A
1162  : NVIDIA L40S-8A
1163  : NVIDIA L40S-12A
1164  : NVIDIA L40S-16A
1165  : NVIDIA L40S-24A
1166  : NVIDIA L40S-48A
2164  : NVIDIA L40S-3B
*/

func fetchAvailableTypes(pciDeviceRoot string, deviceAddress string) (map[string]string, error) {
	creatableVgpuTypesFile := filepath.Join(pciDeviceRoot, deviceAddress, "nvidia", creatableVGPUTypes)
	_, err := os.Stat(creatableVgpuTypesFile)
	if err != nil {
		return nil, fmt.Errorf("could not get file %s for vgpu device %s: %w", creatableVGPUTypes, deviceAddress, err)
	}

	contents, err := os.ReadFile(creatableVgpuTypesFile)
	if err != nil {
		return nil, fmt.Errorf("error reading creatable_vgpu_types_file %w", err)
	}

	availableTypes := parseCurrentVGPUTypes(contents)
	return availableTypes, nil
}

func FetchVGPUStatus(pciDeviceRoot string, deviceAddress string) (*v1beta1.VGPUDeviceStatus, error) {
	currentType, err := fetchCurrentVGPUType(pciDeviceRoot, deviceAddress)
	if err != nil {
		return nil, fmt.Errorf("error during fetchVGPUstatus: %w", err)
	}

	availableTypes, err := fetchAvailableTypes(pciDeviceRoot, deviceAddress)
	if err != nil {
		return nil, fmt.Errorf("error during fetchAvailableTypes: %w", err)
	}

	status := &v1beta1.VGPUDeviceStatus{
		AvailableTypes: availableTypes,
		VGPUStatus:     v1beta1.VGPUDisabled,
	}

	if currentType != "0" && currentType != "" {
		status.UUID = currentType
		status.VGPUStatus = v1beta1.VGPUEnabled
	}
	return status, nil
}

func evalPhysFn(devicePath string) (string, error) {
	physResolvedLink, err := filepath.EvalSymlinks(path.Join(devicePath, "physfn"))
	if err != nil {
		return "", fmt.Errorf("error querying physical function: %v", err)
	}
	physFn := strings.Split(physResolvedLink, "/")
	return physFn[len(physFn)-1], nil
}

func GenerateDeviceName(deviceName string) string {
	return common.GeneratevGPUDeviceName(deviceName)
}

func fetchCurrentVGPUType(basepath, pciAddress string) (string, error) {
	vgpuTypePath := filepath.Join(basepath, pciAddress, "nvidia", currentVGPUType)
	data, err := os.ReadFile(vgpuTypePath)
	if err != nil {
		return "", fmt.Errorf("error reading current_vgpu_type for device %s: %w", pciAddress, err)
	}

	return strings.TrimSpace(string(data)), nil
}

/*
	parses contents of creatable_vgpu_types for a particular vGPU

ID    : vGPU Name
1145  : NVIDIA L40S-1B
1146  : NVIDIA L40S-2B
1147  : NVIDIA L40S-1Q
1148  : NVIDIA L40S-2Q
1149  : NVIDIA L40S-3Q
1150  : NVIDIA L40S-4Q
1151  : NVIDIA L40S-6Q
1152  : NVIDIA L40S-8Q
1153  : NVIDIA L40S-12Q
1154  : NVIDIA L40S-16Q
1155  : NVIDIA L40S-24Q
1156  : NVIDIA L40S-48Q
1157  : NVIDIA L40S-1A
1158  : NVIDIA L40S-2A
1159  : NVIDIA L40S-3A
1160  : NVIDIA L40S-4A
1161  : NVIDIA L40S-6A
1162  : NVIDIA L40S-8A
1163  : NVIDIA L40S-12A
1164  : NVIDIA L40S-16A
1165  : NVIDIA L40S-24A
1166  : NVIDIA L40S-48A
2164  : NVIDIA L40S-3B
*/
func parseCurrentVGPUTypes(contents []byte) map[string]string {
	availableTypes := make(map[string]string)
	lines := strings.Split(string(contents), "\n")
	for _, v := range lines {
		elements := strings.Split(v, ":")
		if len(elements) != 2 {
			continue
		}
		availableTypes[strings.TrimSpace(elements[1])] = strings.TrimSpace(elements[0])
		delete(availableTypes, "vGPU Name") // remove the header of file
	}

	return availableTypes
}
