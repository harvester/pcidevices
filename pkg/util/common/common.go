package common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

func VGPUDeviceByResourceName(obj *v1beta1.VGPUDevice) ([]string, error) {
	return []string{
		GeneratevGPUDeviceName(obj.Status.ConfiguredVGPUTypeName),
	}, nil
}

func GeneratevGPUDeviceName(deviceName string) string {
	deviceName = strings.TrimSpace(deviceName)
	deviceName = strings.ToUpper(deviceName)
	deviceName = strings.Replace(deviceName, "/", "_", -1)
	deviceName = strings.Replace(deviceName, ".", "_", -1)
	//deviceName = strings.Replace(deviceName, "-", "_", -1)
	reg, _ := regexp.Compile(`\s+`)
	deviceName = reg.ReplaceAllString(deviceName, "_")
	// Removes any char other than alphanumeric and underscore
	reg, _ = regexp.Compile(`^a-zA-Z0-9_-.]+`)
	deviceName = reg.ReplaceAllString(deviceName, "")
	return fmt.Sprintf("nvidia.com/%s", deviceName)
}
