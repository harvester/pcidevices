package virtualmachine

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func Test_patchHostDevices(t *testing.T) {
	assert := require.New(t)

	HostDevices := map[string][]string{
		"device.com/sample": {"node1dev1", "node1dev2"},
	}
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       "node1dev1",
									DeviceName: "device.com/sample",
								},
								{
									Name:       "randomDevice",
									DeviceName: "device.com/sample",
								},
							},
						},
					},
				},
			},
		},
	}
	patchHostDevices(vm, HostDevices)
	// expect randomDevice to be replaced with actual device name
	assert.Equal(vm.Spec.Template.Spec.Domain.Devices.HostDevices[1].Name, "node1dev2")

}

func Test_patchGPUDevices(t *testing.T) {
	assert := require.New(t)

	GPUDevices := map[string][]string{
		"nvidia.com/A2-Q2": {"node1dev1"},
	}
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							GPUs: []kubevirtv1.GPU{
								{
									Name:       "sample",
									DeviceName: "nvidia.com/A2-Q2",
								},
							},
						},
					},
				},
			},
		},
	}
	patchGPUDevices(vm, GPUDevices)
	// expect randomDevice to be replaced with actual device name
	assert.Equal(vm.Spec.Template.Spec.Domain.Devices.GPUs[0].Name, "node1dev1")

}

func Test_reconcileGPUDetails(t *testing.T) {
	vGPUMap := map[string]string{
		"e898f311-6b9e-46a2-b728-144d01af1a7c": "vgpu-01-000008013",
		"95242535-e423-41a6-bb58-f28a44d68d66": "vgpu-01-000008010",
	}

	envMap := map[string]string{
		"MDEV_PCI_RESOURCE_NVIDIA_COM_NVIDIA_A2-4Q": "e898f311-6b9e-46a2-b728-144d01af1a7c,95242535-e423-41a6-bb58-f28a44d68d66",
		"KUBERNETES_SERVICE_HOST":                   "10.53.0.1",
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Devices: kubevirtv1.Devices{
					GPUs: []kubevirtv1.GPU{
						{
							Name:       "sample1",
							DeviceName: "nvidia.com/NVIDIA_A2-4Q",
						},
						{
							Name:       "sample2",
							DeviceName: "nvidia.com/NVIDIA_A2-4Q",
						},
					},
				},
			},
		},
	}
	gpuMap := reconcileGPUDetails(vmi, envMap, vGPUMap)
	assert := require.New(t)
	assert.Len(gpuMap["nvidia.com/NVIDIA_A2-4Q"], 2, "expected to find only 2 gpus")
}

func Test_reconcilePCIDeviceDetails(t *testing.T) {
	pciDeviceMap := map[string]string{
		"0000:08:00.2": "harvester-f7gmj-000008002",
		"0000:08:00.3": "harvester-f7gmj-000008003",
	}

	envMap := map[string]string{
		"PCI_RESOURCE_MELLANOX_COM_MT27700_FAMILY_CONNECTX4_VIRTUAL_FUNCTION": "0000:08:00.2,0000:08:00.2,0000:08:00.3,0000:08:00.3",
		"KUBERNETES_SERVICE_HOST": "10.53.0.1",
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Devices: kubevirtv1.Devices{
					HostDevices: []kubevirtv1.HostDevice{
						{
							Name:       "sample1",
							DeviceName: "mellanox.com/MT27700_FAMILY_CONNECTX4_VIRTUAL_FUNCTION",
						},
						{
							Name:       "sample2",
							DeviceName: "mellanox.com/MT27700_FAMILY_CONNECTX4_VIRTUAL_FUNCTION",
						},
					},
				},
			},
		},
	}
	gpuMap := reconcilePCIDeviceDetails(vmi, envMap, pciDeviceMap)
	assert := require.New(t)
	assert.Len(gpuMap["mellanox.com/MT27700_FAMILY_CONNECTX4_VIRTUAL_FUNCTION"], 2, "expected to find only 2 gpus")
}
