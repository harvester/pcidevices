package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

const (
	defaultConfiguredVFFile = "sriov_numvfs"
	defaultVFCheckFile      = "sriov_vf_device"
)

// IsDeviceSRIOVCapable checks for existence of `sriov_vf_device` file in the pcidevice tree
func IsDeviceSRIOVCapable(devicePath string) (bool, error) {
	vfCheckFilePath := filepath.Join(devicePath, defaultVFCheckFile)
	_, err := os.Stat(vfCheckFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	contents, err := os.ReadFile(vfCheckFilePath)
	if err != nil {
		return false, err
	}

	if strings.TrimSuffix(string(contents), "\n") == "0" {
		return false, nil
	}
	return true, nil
}

// CurrentVFConfigured returns number of VF's defined
func CurrentVFConfigured(devicePath string) (int, error) {
	contents, err := os.ReadFile(filepath.Join(devicePath, defaultConfiguredVFFile))
	if err != nil {
		return 0, fmt.Errorf("error reading %s for device %s: %v", defaultConfiguredVFFile, devicePath, err)
	}

	numvfs := strings.Trim(string(contents), "\n")
	return strconv.Atoi(numvfs)
}

// copied from: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/v3.0.0/pkg/utils/utils.go#L112
// with minor change to allow override to sysBusPci path to make it easier to test with umockdev
// GetVFList returns a List containing PCI addr for all VF discovered in a given PF
func GetVFList(pfDir string) (vfList []string, err error) {
	_, err = os.Lstat(pfDir)
	if err != nil {
		err = fmt.Errorf("error: could not get PF directory information for device: %s, Err: %v", pfDir, err)
		return
	}

	vfDirs, err := filepath.Glob(filepath.Join(pfDir, "virtfn*"))

	if err != nil {
		err = fmt.Errorf("error reading VF directories %v", err)
		return
	}

	vfList = make([]string, 0, len(vfDirs))
	//Read all VF directory and get add VF PCI addr to the vfList
	for _, dir := range vfDirs {
		dirInfo, err := os.Lstat(dir)
		if err == nil && (dirInfo.Mode()&os.ModeSymlink != 0) {
			linkName, err := filepath.EvalSymlinks(dir)
			if err == nil {
				vfLink := filepath.Base(linkName)
				vfList = append(vfList, vfLink)
			}
		}
	}
	return
}

func VMBySpecHostDeviceName(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	hostDeviceName := make([]string, 0, len(obj.Spec.Template.Spec.Domain.Devices.HostDevices))
	for _, hostDevice := range obj.Spec.Template.Spec.Domain.Devices.HostDevices {
		hostDeviceName = append(hostDeviceName, hostDevice.Name)
	}
	return hostDeviceName, nil
}

// VMByHostDeviceName indexes VM's by host device name.
// It could be usb device claim or pci device claim name.
func VMByHostDeviceName(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	if obj.Annotations == nil {
		return nil, nil
	}

	allocationDetails, ok := obj.Annotations[v1beta1.DeviceAllocationKey]
	if !ok {
		return nil, nil
	}

	allocatedHostDevices, err := generateHostDeviceAllocation(obj, allocationDetails)
	if err != nil {
		return nil, err
	}

	return allocatedHostDevices, nil
}

// VMByVGPUDevice indexes VM's by vgpu names
func VMByVGPUDevice(obj *kubevirtv1.VirtualMachine) ([]string, error) {
	// find and add vgpu info from the DeviceAllocationKey annotation if present on the vm
	if obj.Annotations == nil {
		return nil, nil
	}
	allocationDetails, ok := obj.Annotations[v1beta1.DeviceAllocationKey]
	if !ok {
		return nil, nil
	}

	allocatedGPUs, err := generateGPUDeviceAllocation(obj, allocationDetails)
	if err != nil {
		return nil, err
	}
	return allocatedGPUs, nil
}

func generateDeviceAllocationDetails(allocationDetails string) (*v1beta1.AllocationDetails, error) {
	currentAllocation := &v1beta1.AllocationDetails{}
	err := json.Unmarshal([]byte(allocationDetails), currentAllocation)
	return currentAllocation, err
}

func generateDeviceInfo(devices map[string][]string) []string {
	total := 0
	for _, v := range devices {
		total += len(v)
	}

	allDevices := make([]string, 0, total)
	for _, v := range devices {
		allDevices = append(allDevices, v...)
	}
	return allDevices
}

func generateGPUDeviceAllocation(obj *kubevirtv1.VirtualMachine, allocationDetails string) ([]string, error) {
	allocation, err := generateDeviceAllocationDetails(allocationDetails)
	if err != nil {
		return nil, fmt.Errorf("error generating device allocation details %s/%s: %v", obj.Name, obj.Namespace, err)
	}

	if allocation.GPUs != nil {
		return generateDeviceInfo(allocation.GPUs), nil
	}
	return nil, nil
}

func generateHostDeviceAllocation(obj *kubevirtv1.VirtualMachine, allocationDetails string) ([]string, error) {
	allocation, err := generateDeviceAllocationDetails(allocationDetails)
	if err != nil {
		return nil, fmt.Errorf("error generating device allocation details %s/%s: %v", obj.Name, obj.Namespace, err)
	}

	if allocation.HostDevices != nil {
		return generateDeviceInfo(allocation.HostDevices), nil
	}
	return nil, nil
}

func USBDeviceByResourceName(obj *v1beta1.USBDevice) ([]string, error) {
	return []string{obj.Status.ResourceName}, nil
}

func VGPUDeviceByResourceName(obj *v1beta1.VGPUDevice) ([]string, error) {
	return []string{
		GeneratevGPUDeviceName(obj.Status.ConfiguredVGPUTypeName),
	}, nil
}

func GeneratevGPUDeviceName(deviceName string) string {
	deviceName = strings.TrimSpace(deviceName)
	//deviceName = strings.ToUpper(deviceName)
	deviceName = strings.ReplaceAll(deviceName, "/", "_")
	deviceName = strings.ReplaceAll(deviceName, ".", "_")
	//deviceName = strings.Replace(deviceName, "-", "_", -1)
	reg, _ := regexp.Compile(`\s+`)
	deviceName = reg.ReplaceAllString(deviceName, "_")
	// Removes any char other than alphanumeric and underscore
	reg, _ = regexp.Compile(`^a-zA-Z0-9_-.]+`)
	deviceName = reg.ReplaceAllString(deviceName, "")
	return fmt.Sprintf("nvidia.com/%s", deviceName)
}
