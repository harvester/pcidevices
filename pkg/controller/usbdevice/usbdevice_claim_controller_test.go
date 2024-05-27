package usbdevice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	mockUsbDevice1 = &v1beta1.USBDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node-0951-1666-001002",
			Labels: map[string]string{
				"nodename": "test-node",
			},
		},
		Status: v1beta1.USBDeviceStatus{
			VendorID:     "0951",
			ProductID:    "1666",
			ResourceName: "kubevirt.io/test-node-0951-1666-001002",
			DevicePath:   "/dev/bus/usb/001/002",
			NodeName:     "test-node",
			Description:  "DataTraveler 100 G3/G4/SE9 G2/50 Kyson (Kingston Technology)",
		},
	}

	mockKubeVirt = &kubevirtv1.KubeVirt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt",
			Namespace: KubeVirtNamespace,
		},
		Spec: kubevirtv1.KubeVirtSpec{},
	}

	mockUsbDeviceClaim1 = &v1beta1.USBDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node-0951-1666-001002",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "devices.harvesterhci.io/v1beta1",
					Kind:       "USBDevice",
					Name:       "test-node-0951-1666-001002",
					UID:        "test-uid",
				},
			},
		},
		Status: v1beta1.USBDeviceClaimStatus{
			NodeName: "test-node",
		},
	}
)

func Test_OnUSBDeviceClaimChanged(t *testing.T) {
	client := fake.NewSimpleClientset(mockUsbDevice1, mockUsbDeviceClaim1, mockKubeVirt)

	handler := NewClaimHandler(
		fakeclients.USBDeviceCache(client.DevicesV1beta1().USBDevices),
		fakeclients.USBDeviceClaimsClient(client.DevicesV1beta1().USBDeviceClaims),
		fakeclients.USBDevicesClient(client.DevicesV1beta1().USBDevices),
		fakeclients.KubeVirtClient(client.KubevirtV1().KubeVirts),
	)

	_, err := handler.OnUSBDeviceClaimChanged("", mockUsbDeviceClaim1)
	assert.NoError(t, err)

	kubevirt, err := client.KubevirtV1().KubeVirts(mockKubeVirt.Namespace).Get(context.Background(), mockKubeVirt.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, kubevirtv1.KubeVirtSpec{
		Configuration: kubevirtv1.KubeVirtConfiguration{
			PermittedHostDevices: &kubevirtv1.PermittedHostDevices{
				USB: []kubevirtv1.USBHostDevice{
					{
						ResourceName:             "kubevirt.io/test-node-0951-1666-001002",
						ExternalResourceProvider: true,
						Selectors: []kubevirtv1.USBSelector{
							{
								Vendor:  "0951",
								Product: "1666",
							},
						},
					},
				},
			},
		},
	}, kubevirt.Spec)
	usbDevice, err := client.DevicesV1beta1().USBDevices().Get(context.Background(), mockUsbDevice1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, true, usbDevice.Status.Enabled)
}

func Test_OnUSBDeviceClaimRemove(t *testing.T) {
	mockUsbDevice1.Status.Enabled = true
	mockKubeVirt.Spec.Configuration = kubevirtv1.KubeVirtConfiguration{
		PermittedHostDevices: &kubevirtv1.PermittedHostDevices{
			USB: []kubevirtv1.USBHostDevice{
				{
					ResourceName:             mockUsbDevice1.Status.ResourceName,
					ExternalResourceProvider: true,
					Selectors: []kubevirtv1.USBSelector{
						{
							Vendor:  "0951",
							Product: "1666",
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(mockUsbDevice1, mockKubeVirt)

	handler := NewClaimHandler(
		fakeclients.USBDeviceCache(client.DevicesV1beta1().USBDevices),
		fakeclients.USBDeviceClaimsClient(client.DevicesV1beta1().USBDeviceClaims),
		fakeclients.USBDevicesClient(client.DevicesV1beta1().USBDevices),
		fakeclients.KubeVirtClient(client.KubevirtV1().KubeVirts),
	)

	_, err := handler.OnRemove("", mockUsbDeviceClaim1)
	assert.NoError(t, err)
	usbDevice, err := client.DevicesV1beta1().USBDevices().Get(context.Background(), mockUsbDevice1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, false, usbDevice.Status.Enabled)
	kubeVirt, err := client.KubevirtV1().KubeVirts(mockKubeVirt.Namespace).Get(context.Background(), mockKubeVirt.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(kubeVirt.Spec.Configuration.PermittedHostDevices.USB))
}
