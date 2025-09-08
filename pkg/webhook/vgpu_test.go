package webhook

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/stretchr/testify/require"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	oldUsedVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu1",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  true,
			NodeName: "node1",
		},
	}

	newUsedVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu1",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  false,
			NodeName: "node1",
		},
	}
	oldFreeVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu2",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  true,
			NodeName: "node1",
		},
	}

	newFreeVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu2",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  false,
			NodeName: "node1",
		},
	}

	oldDisabledVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu1",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  false,
			NodeName: "node1",
		},
		Status: devicesv1beta1.VGPUDeviceStatus{
			AvailableTypes: map[string]string{
				"GRID A100-8C":  "nvidia-470",
				"GRID A100-10C": "nvidia-471",
			},
		},
	}
	newEnabledVGPU = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu1",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  true,
			NodeName: "node1",
		},
	}

	newEnabledVGPUMissingNode = &devicesv1beta1.VGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu1",
		},
		Spec: devicesv1beta1.VGPUDeviceSpec{
			Enabled:  true,
			NodeName: "node2",
		},
	}

	vm1 = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vgpu-vm",
			Namespace: "default",
			Annotations: map[string]string{
				devicesv1beta1.DeviceAllocationKey: `{"gpus":{"nvidia.com/fakevgpu":["vgpu1"]}}`,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							GPUs: []kubevirtv1.GPU{
								{
									Name:       "vgpu1",
									DeviceName: "nvidia.com/fakevgpu",
								},
							},
						},
					},
				},
			},
		},
	}

	vm2 = &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "novgpu-vm",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							GPUs: []kubevirtv1.GPU{},
						},
					},
				},
			},
		},
	}
)

func Test_VGPUUpdates(t *testing.T) {
	var testCases = []struct {
		name        string
		oldVGPU     *devicesv1beta1.VGPUDevice
		newVGPU     *devicesv1beta1.VGPUDevice
		expectError bool
	}{
		{
			name:        "vgpu1 assigned to vm",
			oldVGPU:     oldUsedVGPU,
			newVGPU:     newUsedVGPU,
			expectError: true,
		},
		{name: "vgpu2 not assigned vm",
			oldVGPU:     oldFreeVGPU,
			newVGPU:     newFreeVGPU,
			expectError: false,
		},
	}

	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vm1, vm2)
	virtualMachineCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	vGPUValidator := NewVGPUValidator(virtualMachineCache, nodeCache)
	for _, v := range testCases {
		err := vGPUValidator.Update(nil, v.oldVGPU, v.newVGPU)
		if v.expectError {
			assert.Error(err, fmt.Sprintf("expected to find error for test case %s", v.name))
		} else {
			assert.NoError(err, fmt.Sprintf("expected to find no error for test case %s", v.name))
		}
	}
}

func Test_VGPUDeletion(t *testing.T) {
	var testCases = []struct {
		name        string
		gpu         *devicesv1beta1.VGPUDevice
		expectError bool
	}{
		{
			name:        "vgpu is enabled",
			gpu:         oldUsedVGPU,
			expectError: true,
		},
		{name: "vgpu is disabled",
			gpu:         newFreeVGPU,
			expectError: false,
		},
		{
			name:        "vgpu is enabled but on deleted node",
			gpu:         newEnabledVGPUMissingNode,
			expectError: false,
		},
	}

	assert := require.New(t)
	harvesterfakeClient := harvesterfake.NewSimpleClientset(vm1, vm2)
	virtualMachineCache := fakeclients.VirtualMachineCache(harvesterfakeClient.KubevirtV1().VirtualMachines)
	vGPUValidator := NewVGPUValidator(virtualMachineCache, nodeCache)
	for _, v := range testCases {
		err := vGPUValidator.Delete(nil, v.gpu)
		if v.expectError {
			assert.Error(err, fmt.Sprintf("expected to find error for test case %s", v.name))
		} else {
			assert.NoError(err, fmt.Sprintf("expected to find no errorerror for test case %s", v.name))
		}
	}
}

func Test_validateVGPUProfile(t *testing.T) {
	var testCases = []struct {
		name        string
		gpu         *devicesv1beta1.VGPUDevice
		vGPUProfile string
		expectError bool
	}{
		{
			name:        "no vGPU Profile",
			gpu:         oldDisabledVGPU,
			expectError: true,
		},
		{
			name:        "invalid vGPU Profile",
			gpu:         newFreeVGPU,
			vGPUProfile: "GRID A100-40C",
			expectError: true,
		},
		{
			name:        "valid vGPU Profile",
			gpu:         newFreeVGPU,
			vGPUProfile: "GRID A100-10C",
			expectError: false,
		},
	}

	assert := require.New(t)
	for _, v := range testCases {
		newEnabledVGPUCopy := newEnabledVGPU.DeepCopy()
		newEnabledVGPUCopy.Spec.VGPUTypeName = v.vGPUProfile
		err := validateVGPUProfiles(oldDisabledVGPU, newEnabledVGPUCopy)
		if v.expectError {
			assert.Error(err, fmt.Sprintf("expected to find error for test case: %s", v.name))
		} else {
			assert.NoError(err, fmt.Sprintf("expected to find no error for test case: %s", v.name))
		}
	}
}
