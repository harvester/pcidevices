# Diagnostic commands for working with PCI Devices

Here are some useful commands while working with PCI devices in Harvester.

## Background

When you enable a PCI Device for passthrough, it creates a [PCIDeviceClaim](../pkg/apis/devices.harvesterhci.io/v1beta1/pcideviceclaim.go), then the [PCIDeviceClaim controller](../pkg/controller/pcideviceclaim/pcideviceclaim_controller.go) sees the new claim and then:

### Steps to enable PCI passthrough
1. Get the [PCIDevice](../pkg/apis/devices.harvesterhci.io/v1beta1/pcidevice.go) for the new claim
2. It permits the new host device in KubeVirt
3. It enables PCI passthrough on the device by binding the underlying PCI device to the [vfio-pci driver](https://docs.kernel.org/driver-api/vfio.html)
4. It creates a [DevicePlugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) which uses a [UNIX domain socket](https://en.wikipedia.org/wiki/Unix_domain_socket) to allow [KubeVirt](https://kubevirt.io/) to request devices for VMs.
   - if the device's `resourceName` already has a DevicePlugin, then it [adds](https://github.com/tlehman/pcidevices/blob/7cfa4f2b4ef251efd9e75b78b5db3260aa2dbb6b/pkg/deviceplugins/deviceplugin.go#L63) that device to the existing deviceplugin


# Diagnostics

## 1. How to check if the PCIDeviceClaim object is there?
```shell
% kubectl get pcidevice janus-000004000
NAME              ADDRESS        VENDOR ID   DEVICE ID   NODE NAME   DESCRIPTION                                                                  KERNEL DRIVER IN USE
janus-000004000   0000:04:00.0   10de        1c02        janus       VGA compatible controller: NVIDIA Corporation GP106 [GeForce GTX 1060 3GB]   vfio-pci

```

Notice the `KERNEL DRIVER IN USE` column, if it says `vfio-pci`, then the underlying PCI device is ready for PCI passthrough, assuming that it's true.

But for Harvester to be able to recognize it as enabled, it needs a `PCIDeviceClaim`, which should have the same name as the `PCIDevice`, so run 

```shell
% kubectl get pcideviceclaim janus-000004000
NAME              ADDRESS        NODE NAME   USER NAME   KERNEL DRIVER ΤΟ UNBIND   PASSTHROUGH ENABLED
janus-000004000   0000:04:00.0   janus       admin                                 true
```

The existence of this PCIDeviceClaim with a passthrough enabled value of `true` is sufficient for Harvester to recognize this device is ready for passthrough to a VM.

## 2. How to check the list of permitted devices in KubeVirt

The next diagnostic is checking KubeVirt's config to see if the device has been permitted to be attached to a VM.

```shell
% kubectl get kubevirts.kubevirt.io -n harvester-system kubevirt -o yaml | yq .spec.configuration.permittedHostDevices.pciHostDevices
```
```yaml
- externalResourceProvider: true
  pciVendorSelector: 10de:1c02
  resourceName: nvidia.com/GP106_GEFORCE_GTX_1060_3GB
- externalResourceProvider: true
  pciVendorSelector: 10de:10f1
  resourceName: nvidia.com/GP106_HIGH_DEFINITION_AUDIO_CONTROLLER
```

To get the resourceName of your device, run:

```shell
% kubectl get pcidevice janus-000004000 -o yaml | yq '.status.resourceName'
nvidia.com/GP106_GEFORCE_GTX_1060_3GB
```

So we can see that the device is permitted. If it's not in there, you can work around this by running `kubectl edit kubevirts.kubevirt.io -n harvester-system kubevirt` and just cowboy-editing the `pciHostDevices` yourself. Make sure to set `externalResourceProvider` to true so that our custom deviceplugins are used.

## 3. How to check if the underlying PCI device is prepared for passthrough?

Now, the existence of a `PCIDeviceClaim` object might in principle be incorrect, if some unexpeceted condition occurs where the object becomes stale. To check what the Linux kernel says, get the PCI devices' address and then query `lspci` to see if the device is actually bound to `vfio-pci`


```shell
# Get the PCI address
% kubectl get pcideviceclaim janus-000004000 -o yaml | yq '.spec.address'
0000:04:00.0
# SSH Into the Node
% ssh rancher@$(kubectl get pcideviceclaim janus-000004000 -o yaml | yq '.spec.nodeName')
rancher@janus:~> sudo su
janus:/home/rancher # lspci -s 0000:04:00.0 -v | tail -5
	Capabilities: [420] Advanced Error Reporting
	Capabilities: [600] Vendor Specific Information: ID=0001 Rev=1 Len=024 <?>
	Capabilities: [900] #19
	Kernel driver in use: vfio-pci
```

Notice how it says `vfio-pci` is currently in use. This means that the PCIDeviceClaim's `kernelDriverInUse: "vfio-pci"` entry is correct.

## 4. How to check the DevicePlugin status

DevicePlugins are little programs that manage a set of devices with the same resourceName. In our example, that would be `nvidia.com/GP106_GEFORCE_GTX_1060_3GB`. To make this more concrete, assume you did the ssh step in part 3 above, and you are currently sshed into the node and have root privileges through `sudo su`:


```shell
# Change directory to where the kubelet keeps the device plugins
janus:/home/rancher # cd /var/lib/kubelet/device-plugins/

# Look at all the device plugin sockets: 
janus:/var/lib/kubelet/device-plugins # ls 
DEPRECATION  kubelet.sock  kubelet_internal_checkpoint	kubevirt-kvm.sock  kubevirt-nvidia.com-GP106_GEFORCE_GTX_1060_3GB.sock	kubevirt-nvidia.com-GP106_HIGH_DEFINITION_AUDIO_CONTROLLER.sock  kubevirt-tun.sock  kubevirt-vhost-net.sock
```

Notice the `kubevirt-nvidia.com-GP106_GEFORCE_GTX_1060_3GB.sock` file, that's the socket that the kubelet uses to expose KubeVirt to the local PCI Devices.

The RPC messages that get sent on the socket are:

1. [ListAndWatch](https://github.com/tlehman/pcidevices/blob/7cfa4f2b4ef251efd9e75b78b5db3260aa2dbb6b/pkg/deviceplugins/device_manager.go#L212) to see which devices are available
2. [Allocate](https://github.com/tlehman/pcidevices/blob/7cfa4f2b4ef251efd9e75b78b5db3260aa2dbb6b/pkg/deviceplugins/device_manager.go#L251) to take a device and attach it to a VM

Those two methods do the bulk of the work on the DevicePlugin side. The other way to look at if the deviceplugins are behaving is by checking the node status:

```shell
% kubectl get nodes janus -o yaml | yq .status.capacity
cpu: "8"
devices.kubevirt.io/kvm: 1k
devices.kubevirt.io/tun: 1k
devices.kubevirt.io/vhost-net: 1k
ephemeral-storage: 102626232Ki
hugepages-2Mi: "0"
memory: 24575392Ki
nvidia.com/GP106_GEFORCE_GTX_1060_3GB: "1"
nvidia.com/GP106_HIGH_DEFINITION_AUDIO_CONTROLLER: "1"
pods: "110"
```

Notice the `resourceName` on the left and the count on the right. That shows the deviceplugin status. If you had two GTX 1060 cards on that node, then when the second one was enabled, it should look like `nvidia.com/GP106_GEFORCE_GTX_1060_3GB: "1"`

Finally, the capacity just shows the number of devices, but when KubeVirt calls `Allocate` (see above) to attach the device to a VM, the `.status.allocatable` needs to be nonzero, here's how to check that:

```shell
 % kubectl get nodes janus -o yaml | yq .status.allocatable
cpu: "8"
devices.kubevirt.io/kvm: 1k
devices.kubevirt.io/tun: 1k
devices.kubevirt.io/vhost-net: 1k
ephemeral-storage: "99834798412"
hugepages-2Mi: "0"
memory: 24575392Ki
nvidia.com/GP106_GEFORCE_GTX_1060_3GB: "1"
nvidia.com/GP106_HIGH_DEFINITION_AUDIO_CONTROLLER: "1"
pods: "110"
```
