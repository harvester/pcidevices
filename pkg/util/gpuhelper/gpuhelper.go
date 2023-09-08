package gpuhelper

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
)

const (
	supportedTypesDir      = "mdev_supported_types"
	availableTypesFileName = "available_instances"
	vGPUNameFile           = "name"
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
		if v.IsGPU() && v.IsVF {
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

	status, err := FetchVGPUStatus(v1beta1.MdevRoot, v1beta1.SysDevRoot, v1beta1.MdevBusClassRoot, device.Address)
	if err != nil {
		return nil, err
	}
	if status.ConfiguredVGPUTypeName != "" {
		vgpu.Spec.Enabled = true
	}
	vgpu.Status = *status
	return vgpu, nil
}

// fetchAvailable is equivalent to running
// /sys/class/mdev_bus/0000:08:02.0/mdev_supported_types # for i in *; do echo "$i" $(cat $i/name) available: $(cat $i/avail*); done
// nvidia-742 NVIDIA A2-1B available: 0
// nvidia-743 NVIDIA A2-2B available: 0
// nvidia-744 NVIDIA A2-1Q available: 0
// nvidia-745 NVIDIA A2-2Q available: 0
// nvidia-746 NVIDIA A2-4Q available: 1
// nvidia-747 NVIDIA A2-8Q available: 0
// nvidia-748 NVIDIA A2-16Q available: 0
// nvidia-749 NVIDIA A2-1A available: 0
// nvidia-750 NVIDIA A2-2A available: 0
// nvidia-751 NVIDIA A2-4A available: 1
// nvidia-752 NVIDIA A2-8A available: 0
// nvidia-753 NVIDIA A2-16A available: 0
// nvidia-754 NVIDIA A2-4C available: 1
// nvidia-755 NVIDIA A2-8C available: 0
// nvidia-756 NVIDIA A2-16C available: 0
// it will only return device types which are available, which can then be used to define vGPU type
func fetchAvailableTypes(managedBusPath string, deviceAddress string) (map[string]string, error) {
	vGPUTypesDir := filepath.Join(managedBusPath, deviceAddress, supportedTypesDir)
	_, err := os.Lstat(vGPUTypesDir)
	if err != nil {
		return nil, fmt.Errorf("could not get %s directory information for device: %s, err: %v", supportedTypesDir, deviceAddress, err)
	}

	vgpuTypes, err := filepath.Glob(filepath.Join(vGPUTypesDir, "nvidia*"))
	if err != nil {
		return nil, fmt.Errorf("error querying supported types: %v", err)
	}

	availableTypes := make(map[string]string)
	for _, dir := range vgpuTypes {
		availableInstances := filepath.Join(dir, availableTypesFileName)
		_, err := os.Stat(availableInstances)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		availableInstancesContent, err := os.ReadFile(availableInstances)
		if err != nil {
			return nil, err
		}
		if strings.Trim(string(availableInstancesContent), "\n") == "1" {
			nameFile := filepath.Join(dir, vGPUNameFile)
			nameContent, err := os.ReadFile(nameFile)
			if err != nil {
				return nil, err
			}
			dirs := strings.Split(dir, "/")
			availableTypes[strings.Trim(string(nameContent), "\n")] = dirs[len(dirs)-1]
		}
	}
	return availableTypes, nil
}

func FetchVGPUStatus(mdevRoot string, pciDeviceRoot string, managedBusPath string, deviceAddress string) (*v1beta1.VGPUDeviceStatus, error) {
	var uuid, deviceType string

	// on node reboot mdevRoot will not exist as a result the walk will fail.
	// we need to return an empty status to allow vgpu config to be re-run
	if _, err := os.Stat(mdevRoot); os.IsNotExist(err) {
		return &v1beta1.VGPUDeviceStatus{}, nil
	}

	err := filepath.WalkDir(mdevRoot, func(path string, d fs.DirEntry, err error) error {
		logrus.Debugf("checking path %s", path)
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %s: %v", path, err)
		}

		dirInfo, err := d.Info()
		if err != nil {
			return err
		}

		if dirInfo.Mode().Type() != fs.ModeSymlink {
			// no further action needed, as expected the device to point to a symlink
			return nil
		}

		resolvedSymLink, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("error resolving symlink %s:%v", path, err)
		}

		// symlink doesn't contain pcidevice address. no further action needed
		if !strings.Contains(resolvedSymLink, deviceAddress) {
			return nil
		}

		// return uuid of vGPU
		uuidPaths := strings.Split(path, "/")
		uuid = uuidPaths[len(uuidPaths)-1]

		logrus.Debugf("found device %s", resolvedSymLink)
		/// read configured DeviceType
		mdevFile := filepath.Join(resolvedSymLink, "mdev_type")
		mdevFileSymLink, err := filepath.EvalSymlinks(mdevFile)
		if err != nil {
			return fmt.Errorf("error resolving symlink %s: %v", mdevFileSymLink, err)
		}

		devtypePaths := strings.Split(mdevFileSymLink, "/")
		deviceType = devtypePaths[len(devtypePaths)-1]
		// return
		return nil
	})

	if err != nil {
		return nil, err
	}

	status := &v1beta1.VGPUDeviceStatus{}
	if uuid != "" {
		status.UUID = uuid
		status.VGPUStatus = v1beta1.VGPUEnabled
	}

	if deviceType != "" {
		vGPUName, err := os.ReadFile(filepath.Join(pciDeviceRoot, deviceAddress, supportedTypesDir, deviceType, "name"))
		if err != nil {
			return nil, fmt.Errorf("error reading name for VGPU device %s: %v", deviceAddress, err)
		}
		status.ConfiguredVGPUTypeName = strings.Trim(string(vGPUName), "\n")
	}

	availableTypes, err := fetchAvailableTypes(managedBusPath, deviceAddress)
	if err != nil {
		return nil, err
	}

	status.AvailableTypes = availableTypes
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
