package gpuhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"
	"github.com/stretchr/testify/require"
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
	mockPath := filepath.Join("./testdata", v1beta1.SysDevRoot)
	availableTypes, err := fetchAvailableTypes(mockPath, "0000:26:00.4")
	assert.NoError(err, "expected no error")
	assert.Len(availableTypes, 23, "expected to find 23 available types from fake /sys tree")
}

func Test_fetchVGPUStatus(t *testing.T) {
	assert := require.New(t)
	mockPath := filepath.Join("./testdata", v1beta1.SysDevRoot)
	status, err := FetchVGPUStatus(mockPath, "0000:26:00.4")
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
