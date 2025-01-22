PCI Devices Controller
========
[![Build Status](https://drone-publish.rancher.io/api/badges/harvester/pcidevices/status.svg)](https://drone-publish.rancher.io/harvester/pcidevices) [![Go Report Card](https://goreportcard.com/badge/github.com/harvester/pcidevices)](https://goreportcard.com/report/github.com/harvester/pcidevices) 

PCI Devices Controller is a **Kubernetes controller** that:

- Discovers PCI Devices for nodes in your cluster and
- Allows users to prepare devices for [PCI Passthrough](https://kubevirt.io/user-guide/virtual_machines/host-devices/), 
  for use with KubeVirt-managed virtual machines.

# API 

This operator introduces these CRDs:
- PCIDevice
- PCIDeviceClaim
- USBDevice
- USBDeviceClaim

It introduces different types of devices that can be used here. In general, when a device is claimed, it is marked as "healthy" and is ready to be used. Each device type has own custom device plugin to manage the device lifecycle.

- PCI Device: Each resource name might have multiple PCI devices.
- USB Device: Each resource name only has one USB device.


## PCIDevice

This custom resource represents PCI Devices on the host. 
The motivation behind getting a list of PCIDevice objects for a node is to
have a cloud-native equivalent to the `lspci` command.

For example, if I have a 3 node cluster:

```shell
NAME     STATUS   ROLES                       AGE   VERSION
node1    Ready    control-plane,etcd,master   26h   v1.24.3+k3s1
node2    Ready    control-plane,etcd,master   26h   v1.24.3+k3s1
node3    Ready    control-plane,etcd,master   26h   v1.24.3+k3s1
```

And I wanted to see which PCI Devices were on `node1`, I would have to use ssh and get it like this:

```
user@host % ssh node1
user@node1 ~$ lspci
00:1c.0 PCI bridge: Intel Corporation Device 06b8 (rev f0)
00:1c.7 PCI bridge: Intel Corporation Device 06bf (rev f0)
00:1d.0 PCI bridge: Intel Corporation Comet Lake PCI Express Root Port #9 (rev f0)
00:1f.0 ISA bridge: Intel Corporation Device 068e
```

But as more nodes are added to the cluster, this kind of manual work gets tedious. The
solution is to have a DaemonSet that runs `lspci` on each node and then synchronizes the results 
with the rest of the cluster.

### CRD

The PCIDevice CR looks like this:

```yaml
apiVersion: devices.harvesterhci.io/v1beta1
kind: PCIDevice
metadata:
  name: pcidevice-sample
status:
  address: "00:1f.6"
  vendorId: "8086"
  deviceId: "0d4c"
  classId: "0200"
  nodeName: "titan"
  resourceName: "intel.com/ETHERNET_CONNECTION_11_I219LM"
  description: "Ethernet controller: Intel Corporation Ethernet Connection (11) I219-LM"
  kernelDriverInUse: "e1000e"
```


## PCIDeviceClaim

This custom resource is created to store the request to prepare a device for 
PCI Passthrough. It has `pciAddress` and a `nodeSystemUUID`, since each request is unique 
for a device on a particular node.

### CRD 

The PCIDeviceClaim CR looks like this:

```yaml
apiVersion: devices.harvesterhci.io/v1beta1
kind: PCIDeviceClaim
metadata:
  name: pcideviceclaim-sample
spec:
  address: "00:1f.6"
  nodeName:  "titan"
  userName:  "yuri"
status:
  kernelDriverToUnbind: "e1000e"
  passthroughEnabled: true
```

The PCIDeviceClaim is created with a target PCI address, for the device 
that the user wants to prepare for PCI Passthrough. Then the 
`status.passthroughEnabled` is set to `false` while it's in progress, 
then `true` when it is bound to the `vfio-pci` driver.

The `status.kernelDriverToUnbind` is stored so that deleting the claim 
can re-bind the device to the original driver.

## USBDevice

Collect all USB devices on all nodes, provided to users, so they know how many USB devices they could use.

Considering the same vendor/product case, we should add the bus and device number as a suffix to avoid misunderstanding. It's 002/004 in this case.

### CRD

```yaml
apiVersion: devices.harvesterhci.io/v1beta1
kind: USBDevice
metadata:
  labels:
    nodename: jacknode
  name: jacknode-0951-1666-002004
status:
  description: DataTraveler 100 G3/G4/SE9 G2/50 Kyson (Kingston Technology)
  devicePath: /dev/bus/usb/002/004
  enabled: false
  nodeName: jacknode
  pciAddress: "0000:02:01.0"
  productID: "1666"
  resourceName: kubevirt.io/jacknode-0951-1666-002004
  vendorID: "0951"
```

## USBDeviceClaim

When users decide which USB devices they want to use, they will create a `usbdeviceclaim`. After `usbdeviceclaim` is created, the controller will update spec.configuration.permittedHostDevices.usb in the kubevirt resource.

We use the pciAddress field to prevent users from enabling the specified PCI device.

### CRD

```yaml
apiVersion: devices.harvesterhci.io/v1beta1
kind: USBDeviceClaim
metadata:
  name: jacknode-0951-1666-002004
  ownerReferences:
    - apiVersion: devices.harvesterhci.io/v1beta1
      kind: USBDevice
      name: jacknode-0951-1666-002004
      uid: b584d7d0-0820-445e-bc90-4ffbfe82d63b
spec: {}
status:
  userName: ""
  nodeName: jacknode
  pciAddress: "0000:02:01.0"
```

# Controllers 

There is a DaemonSet that runs the device controller on each node. The controller reconciles the stored list of devices for that node to the actual current list of devices for that node.


## PCIDevice Controller

The PCIDevice controller will pick up on the new currently active driver automatically, as part of its normal operation.

## PCIDeviceClaim Controller

The PCIDeviceClaim controller will process the requests by attempting to set up devices for PCI Passthrough. The steps involved are:
- Load `vfio-pci` kernel module
- Unbind current driver from device
- Create a driver_override for the device
- Bind the `vfio-pci` driver to the device

Once the device is confirmed to have been bound to `vfio-pci`, the PCIDeviceClaim controller will delete the request.

## USBDevice Controller

The USBDevice Controller triggers the collection of USB devices on the host at two specific times:

- When the pod starts.
- When there is a change in the USB devices (including plugin and removal), primarily detected through fsnotify.
 
Once the USBDevice Controller detects the USB devices on the host, it converts them into the USBDevice CRD format and creates the corresponding resources, ignoring any devices that have already been created.

## USBDeviceClaim Controller

The USBDeviceClaim Controller is triggered when a user decides which USB device to use and creates a USB device claim. Upon the creation of a USBDeviceClaim, this controller performs two main actions:

- It activates the device plugin.
- It updates the kubevirt resource.

This controller primarily manages the lifecycle of the device plugin, ensuring that the claimed USB device is properly integrated and available for use.

# Limitation

## USB Device

- Don't support live migration.
- Don't support hot-plug (including re-plug).
- Require re-creating a USBDeviceClaim to enable the USB device if device path changes in following situations:
  - Re-plugging USB device.
  - Rebooting the node.

# Daemon

The daemon will run on each node in the cluster and build up the PCIDevice list. A daemonset will enforce this daemon is 
running on each node.

# Alternatives considered
## [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
NFD detects all kinds of features, like CPU features, USB devices, PCI devices, etc. It needs to be 
configured, and the output is a node label that tells whether a given device is present or not.

This only detects the presence or absence of device, not the number of them.

```json
  "feature.node.kubernetes.io/pci-<device label>.present": "true",
```

Another reason not to use these simple labels is that we want to be able to allow our customers to set custom RBAC rules that restrict who can use which device in the cluster. We can do that with a custom `PCIDevice` CRD, but it's not clear how to do that with node labels.

## License
Copyright (c) 2025 [SUSE, LLC.](https://www.suse.com/)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
