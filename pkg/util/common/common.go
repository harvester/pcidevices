package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultTotalVFFile      = "sriov_totalvfs"
	defaultConfiguredVFFile = "sriov_numvfs"
)

// IsDeviceSRIOVCapable checks for existence of `sriov_totalvfs` file in the pcidevice tree
func IsDeviceSRIOVCapable(devicePath string) (bool, error) {
	_, err := os.Stat(filepath.Join(devicePath, defaultTotalVFFile))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
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
