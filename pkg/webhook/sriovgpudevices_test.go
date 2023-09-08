package webhook

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	oldInUseGPU = &devicesv1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inuse-gpu1",
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			Enabled: true,
		},
		Status: devicesv1beta1.SRIOVGPUDeviceStatus{
			VGPUDevices: []string{
				"vgpu1",
			},
		},
	}

	newInUseGPU = &devicesv1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inuse-gpu1",
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			Enabled: false,
		},
		Status: devicesv1beta1.SRIOVGPUDeviceStatus{
			VGPUDevices: []string{
				"vgpu1",
			},
		},
	}

	oldFreeGPU = &devicesv1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "free-gpu2",
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			Enabled: true,
		},
		Status: devicesv1beta1.SRIOVGPUDeviceStatus{
			VGPUDevices: []string{
				"vgpu2",
			},
		},
	}

	newFreeGPU = &devicesv1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "free-gpu2",
		},
		Spec: devicesv1beta1.SRIOVGPUDeviceSpec{
			Enabled: false,
		},
		Status: devicesv1beta1.SRIOVGPUDeviceStatus{
			VGPUDevices: []string{
				"vgpu2",
			},
		},
	}
)

func Test_SriovGPUUpdate(t *testing.T) {
	var testCases = []struct {
		name        string
		oldSRIOVGPU *devicesv1beta1.SRIOVGPUDevice
		newSRIOVGPU *devicesv1beta1.SRIOVGPUDevice
		expectError bool
	}{
		{
			name:        "vgpu1 assigned to vm",
			oldSRIOVGPU: oldInUseGPU,
			newSRIOVGPU: newInUseGPU,
			expectError: true,
		},
		{name: "vgpu2 not assigned vm",
			oldSRIOVGPU: oldFreeGPU,
			newSRIOVGPU: newFreeGPU,
			expectError: false,
		},
	}

	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vm1, vm2)
	virtualMachineCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	sriovGPUValidator := NewSRIOVGPUValidator(virtualMachineCache)
	for _, v := range testCases {
		err := sriovGPUValidator.Update(nil, v.oldSRIOVGPU, v.newSRIOVGPU)
		if v.expectError {
			assert.Error(err, fmt.Sprintf("expected to find error for test case %s", v.name))
		} else {
			assert.NoError(err, fmt.Sprintf("expected to find no errorerror for test case %s", v.name))
		}
	}
}

func Test_SriovGPUDelete(t *testing.T) {
	var testCases = []struct {
		name        string
		gpu         *devicesv1beta1.SRIOVGPUDevice
		expectError bool
	}{
		{
			name:        "gpu is enabled",
			gpu:         oldInUseGPU,
			expectError: true,
		},
		{name: "gpu is disabled",
			gpu:         newFreeGPU,
			expectError: false,
		},
	}

	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vm1, vm2)
	virtualMachineCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	sriovGPUValidator := NewSRIOVGPUValidator(virtualMachineCache)
	for _, v := range testCases {
		err := sriovGPUValidator.Delete(nil, v.gpu)
		if v.expectError {
			assert.Error(err, fmt.Sprintf("expected to find error for test case %s", v.name))
		} else {
			assert.NoError(err, fmt.Sprintf("expected to find no errorerror for test case %s", v.name))
		}
	}
}
