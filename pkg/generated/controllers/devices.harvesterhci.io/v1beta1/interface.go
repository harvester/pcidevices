/*
Copyright 2022 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/schemes"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	schemes.Register(v1beta1.AddToScheme)
}

type Interface interface {
	Node() NodeController
	PCIDevice() PCIDeviceController
	PCIDeviceClaim() PCIDeviceClaimController
	SRIOVGPUDevice() SRIOVGPUDeviceController
	SRIOVNetworkDevice() SRIOVNetworkDeviceController
	VGPUDevice() VGPUDeviceController
}

func New(controllerFactory controller.SharedControllerFactory) Interface {
	return &version{
		controllerFactory: controllerFactory,
	}
}

type version struct {
	controllerFactory controller.SharedControllerFactory
}

func (c *version) Node() NodeController {
	return NewNodeController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "Node"}, "nodes", false, c.controllerFactory)
}
func (c *version) PCIDevice() PCIDeviceController {
	return NewPCIDeviceController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "PCIDevice"}, "pcidevices", false, c.controllerFactory)
}
func (c *version) PCIDeviceClaim() PCIDeviceClaimController {
	return NewPCIDeviceClaimController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "PCIDeviceClaim"}, "pcideviceclaims", false, c.controllerFactory)
}
func (c *version) SRIOVGPUDevice() SRIOVGPUDeviceController {
	return NewSRIOVGPUDeviceController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "SRIOVGPUDevice"}, "sriovgpudevices", false, c.controllerFactory)
}
func (c *version) SRIOVNetworkDevice() SRIOVNetworkDeviceController {
	return NewSRIOVNetworkDeviceController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "SRIOVNetworkDevice"}, "sriovnetworkdevices", false, c.controllerFactory)
}
func (c *version) VGPUDevice() VGPUDeviceController {
	return NewVGPUDeviceController(schema.GroupVersionKind{Group: "devices.harvesterhci.io", Version: "v1beta1", Kind: "VGPUDevice"}, "vgpudevices", false, c.controllerFactory)
}
