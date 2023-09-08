package fakeclients

import (
	"context"

	kubevirtv1 "github.com/harvester/harvester/pkg/generated/clientset/versioned/typed/kubevirt.io/v1"
	kubevirtctlv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kubevirtv1api "kubevirt.io/api/core/v1"
)

const (
	VMByPCIDeviceClaim = "harvesterhci.io/vm-by-pcideviceclaim"
	VMByVGPU           = "harvesterhci.io/vm-by-vgpu"
)

type VirtualMachineClient func(string) kubevirtv1.VirtualMachineInterface

func (c VirtualMachineClient) Update(virtualMachine *kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	return c(virtualMachine.Namespace).Update(context.TODO(), virtualMachine, metav1.UpdateOptions{})
}

func (c VirtualMachineClient) Get(namespace, name string, options metav1.GetOptions) (*kubevirtv1api.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, options)
}

func (c VirtualMachineClient) Create(virtualMachine *kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	return c(virtualMachine.Namespace).Create(context.TODO(), virtualMachine, metav1.CreateOptions{})
}

func (c VirtualMachineClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}

func (c VirtualMachineClient) List(_ string, _ metav1.ListOptions) (*kubevirtv1api.VirtualMachineList, error) {
	panic("implement me")
}

func (c VirtualMachineClient) UpdateStatus(*kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	panic("implement me")
}

func (c VirtualMachineClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c VirtualMachineClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *kubevirtv1api.VirtualMachine, err error) {
	panic("implement me")
}

type VirtualMachineCache func(string) kubevirtv1.VirtualMachineInterface

func (c VirtualMachineCache) Get(namespace, name string) (*kubevirtv1api.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c VirtualMachineCache) List(namespace string, selector labels.Selector) ([]*kubevirtv1api.VirtualMachine, error) {
	vmList, err := c(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*kubevirtv1api.VirtualMachine, 0, len(vmList.Items))
	for _, node := range vmList.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (c VirtualMachineCache) AddIndexer(_ string, _ kubevirtctlv1.VirtualMachineIndexer) {
	panic("implement me")
}

func (c VirtualMachineCache) GetByIndex(indexName, key string) ([]*kubevirtv1api.VirtualMachine, error) {
	switch indexName {
	case VMByPCIDeviceClaim:
		var vms []*kubevirtv1api.VirtualMachine
		vmList, err := c.List("", labels.NewSelector())
		if err != nil {
			return nil, err
		}

		for _, vm := range vmList {
			for _, hostDevice := range vm.Spec.Template.Spec.Domain.Devices.HostDevices {
				if hostDevice.Name == key {
					vms = append(vms, vm)
				}
			}
		}
		return vms, nil
	case VMByVGPU:
		var vms []*kubevirtv1api.VirtualMachine
		vmList, err := c.List("", labels.NewSelector())
		if err != nil {
			return nil, err
		}

		for _, vm := range vmList {
			for _, gpuDevice := range vm.Spec.Template.Spec.Domain.Devices.GPUs {
				if gpuDevice.Name == key {
					vms = append(vms, vm)
				}
			}
		}
		return vms, nil
	default:
		panic("implement me")
	}
}
