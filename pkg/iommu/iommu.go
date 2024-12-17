package iommu

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func GroupMapForPCIDevices(groupPaths []string) map[string]int {
	groupMap := make(map[string]int)
	for _, groupPath := range groupPaths {
		groupPathSplit := strings.Split(groupPath, "/")
		if len(groupPathSplit) > 6 {
			deviceAddr := groupPathSplit[6]
			group := groupPathSplit[4]
			groupInt, err := strconv.Atoi(group)
			if err == nil {
				groupMap[deviceAddr] = groupInt
			} else {
				logrus.Errorf("groupPath %s contains an invalid IOMMU Group: %v", groupPath, err)
			}
		} else {
			logrus.Fatalf("groupPath %s does not contain a valid PCI address", groupPath)
		}
	}
	return groupMap
}

const sysKernelIommuGroups = "/sys/kernel/iommu_groups"

// return all paths like /sys/kernel/iommu_groups/$GROUP/devices/$DEVICE
func GroupPaths() ([]string, error) {
	// list all iommu groups
	iommuGroups, err := os.ReadDir(sysKernelIommuGroups)
	if err != nil {
		// TODO log the error
		return []string{}, err
	}
	var groupPaths []string
	for _, group := range iommuGroups {
		path := fmt.Sprintf("%s/%s/devices", sysKernelIommuGroups, group.Name())
		devices, err := os.ReadDir(path)
		if err != nil {
			return []string{}, err
		}
		for _, device := range devices {
			groupPath := fmt.Sprintf("%s/%s/devices/%s", sysKernelIommuGroups, group.Name(), device.Name())
			groupPaths = append(groupPaths, groupPath)
		}
	}
	return groupPaths, nil
}

func GetGroupMap(address string) (string, error) {
	iommuGroupPaths, err := GroupPaths()
	if err != nil {
		return "", err
	}

	iommuGroupMap := GroupMapForPCIDevices(iommuGroupPaths)
	if group, found := iommuGroupMap[address]; found {
		return strconv.Itoa(group), nil
	}

	return "", fmt.Errorf("missing group for address: %s", address)
}
