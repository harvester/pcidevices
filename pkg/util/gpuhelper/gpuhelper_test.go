package gpuhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"

	"github.com/stretchr/testify/require"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
)

func Test_IdentifySRIOVGPU(t *testing.T) {
	mockPath := os.Getenv("UMOCKDEV_DIR")
	options := []nvpci.Option{}
	if mockPath != "" {
		pcidevicePath := filepath.Join(mockPath, nvpci.PCIDevicesRoot)
		options = append(options, nvpci.WithPCIDevicesRoot(pcidevicePath))
	}
	assert := require.New(t)
	devs, err := IdentifySRIOVGPU(options, "mocknode")
	assert.NoError(err, "expected no error while querying GPU devices")
	assert.Len(devs, 1, "expected to find atleast 1 GPU from packaged snapshot")
}

func Test_fetchAvailableTypes(t *testing.T) {
	assert := require.New(t)
	mockPath := filepath.Join("./testdata", v1beta1.MdevBusClassRoot)
	availableTypes, err := fetchAvailableTypes(mockPath, "0000:08:01.7")
	assert.NoError(err, "exepcted no error")
	assert.Len(availableTypes, 3, "expected to find 3 available types from fake /sys tree")
}

func Test_fetchVGPUStatus(t *testing.T) {
	assert := require.New(t)
	mockPath := os.Getenv("UMOCKDEV_DIR")
	var mdevRoot, pciDeviceRoot string
	if mockPath != "" {
		mdevRoot = filepath.Join(mockPath, v1beta1.MdevRoot)
		pciDeviceRoot = filepath.Join(mockPath, v1beta1.SysDevRoot)
	}
	managedBusPath := filepath.Join("./testdata", v1beta1.MdevBusClassRoot)
	status, err := FetchVGPUStatus(mdevRoot, pciDeviceRoot, managedBusPath, "0000:08:01.7")
	assert.NoError(err, "expected no error while generating vGPU status")
	assert.NotEmpty(status.AvailableTypes, "expected AvailableTypes to not be empty")
}

func Test_GenerateDeviceName(t *testing.T) {
	assert := require.New(t)
	deviceName := "NVIDIA A2-4C"
	generatedDeviceName := GenerateDeviceName(deviceName)
	t.Log(generatedDeviceName)
	assert.Equal(generatedDeviceName, "nvidia.com/NVIDIA_A2-4C")
}
