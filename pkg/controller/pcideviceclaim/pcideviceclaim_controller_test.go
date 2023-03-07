package pcideviceclaim

import (
	"reflect"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

func TestHandler_getOrphanedPCIDevices(t *testing.T) {
	type args struct {
		nodename string
		pdcs     *v1beta1.PCIDeviceClaimList
		pds      *v1beta1.PCIDeviceList
	}
	orphanpd := v1beta1.PCIDevice{
		ObjectMeta: v1.ObjectMeta{
			Name: "testnode1-00003f062",
			UID:  "450a6607-b836-46fe-9ced-c23cb2cfdef0",
		},
		Status: v1beta1.PCIDeviceStatus{
			Address:           "0000:3f:06.2",
			KernelDriverInUse: "vfio-pci",
			NodeName:          "testnode1",
		},
	}
	pd := v1beta1.PCIDevice{
		ObjectMeta: v1.ObjectMeta{
			Name: "testnode1-00003f063",
		},
		Status: v1beta1.PCIDeviceStatus{
			Address:           "0000:3f:06.3",
			KernelDriverInUse: "vfio-pci",
			NodeName:          "testnode1",
		},
	}
	pdc := v1beta1.PCIDeviceClaim{
		ObjectMeta: v1.ObjectMeta{
			Name: "testnode1-00003f063",
			OwnerReferences: []v1.OwnerReference{
				v1.OwnerReference{
					Kind: "PCIDevice",
					Name: "testnode1-00003f063",
					UID:  pd.GetObjectMeta().GetUID(),
				},
			},
		},
	}

	tests := []struct {
		name    string
		args    args
		want    *v1beta1.PCIDeviceList
		wantErr bool
	}{
		{
			name: "One PCIDevice bound to vfio-pci and zero PCIDeviceClaims",
			args: args{
				nodename: "testnode1",
				pdcs:     &v1beta1.PCIDeviceClaimList{},
				pds: &v1beta1.PCIDeviceList{
					Items: []v1beta1.PCIDevice{orphanpd}},
			},
			want:    &v1beta1.PCIDeviceList{Items: []v1beta1.PCIDevice{orphanpd}},
			wantErr: false,
		},
		{
			name: "Two PCIDevices bound to vfio-pci and one PCIDeviceClaim",
			args: args{
				nodename: "testnode1",
				pdcs: &v1beta1.PCIDeviceClaimList{
					Items: []v1beta1.PCIDeviceClaim{
						pdc,
					},
				},
				pds: &v1beta1.PCIDeviceList{
					Items: []v1beta1.PCIDevice{
						orphanpd, // this should be returned
						pd,       // this should not be returned, since it's claimed above
					},
				},
			},
			want: &v1beta1.PCIDeviceList{
				Items: []v1beta1.PCIDevice{
					orphanpd,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getOrphanedPCIDevices(tt.args.pdcs, tt.args.pds, tt.args.nodename)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handler.getOrphanedPCIDevices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handler.getOrphanedPCIDevices() = %v, \nwant %v", got, tt.want)
			}
		})
	}
}

// IOMMU Groups need to be kept together on the same VM
func Test_getIommuSiblings(t *testing.T) {
	type args struct {
		pd  *v1beta1.PCIDevice
		pds []*v1beta1.PCIDevice // other devices (includes all siblings)
	}

	// Goal of this test: make PCIDeviceClaims for both devices in
	// IOMMU group 14, but not in 23
	iommuGroupMap := map[string]int{
		"0000:04:00.0": 14,
		"0000:04:00.1": 14,
		"0000:24:00.3": 23,
	}

	// All PCIDevices on the same node as the given device
	pds := []*v1beta1.PCIDevice{
		&v1beta1.PCIDevice{ // GPU
			ObjectMeta: v1.ObjectMeta{
				Name: "janus-000004000",
			},
			Status: v1beta1.PCIDeviceStatus{
				Address:      "0000:04:00.0",
				ClassId:      "0300",
				Description:  "VGA compatible controller: NVIDIA Corporation GP106 [GeForce GTX 1060 3GB]",
				DeviceId:     "1c02",
				IOMMUGroup:   "14",
				NodeName:     "janus",
				ResourceName: "nvidia.com/GP106_GEFORCE_GTX_1060_3GB",
				VendorId:     "10de",
			},
		},
		&v1beta1.PCIDevice{ // GPU's Audio Device (same IOMMU group)
			ObjectMeta: v1.ObjectMeta{
				Name: "janus-000004001",
			},
			Status: v1beta1.PCIDeviceStatus{
				Address:           "0000:04:00.1",
				ClassId:           "0403",
				Description:       "Audio device: NVIDIA Corporation GP106 High Definition Audio Controller",
				DeviceId:          "10f1",
				IOMMUGroup:        "14",
				KernelDriverInUse: "snd_hda_intel",
				NodeName:          "janus",
				ResourceName:      "nvidia.com/GP106_HIGH_DEFINITION_AUDIO_CONTROLLER",
				VendorId:          "10de",
			},
		},
		&v1beta1.PCIDevice{ // device in different IOMMU Group
			ObjectMeta: v1.ObjectMeta{
				Name: "janus-000024003",
			},
			Status: v1beta1.PCIDeviceStatus{
				Address:      "0000:24:00.3",
				ClassId:      "0C80",
				Description:  "Class 0C80: NVIDIA Corporation TU116 USB Type-C UCSI Controller",
				DeviceId:     "1aed",
				IOMMUGroup:   "23",
				NodeName:     "janus",
				ResourceName: "nvidia.com/TU116_USB_TYPEC_UCSI_CONTROLLER",
				VendorId:     "10de",
			},
		},
	}

	tests := []struct {
		name string
		args args
		want []*v1beta1.PCIDevice
	}{
		{
			name: "nvidia card with audio device",
			args: args{
				pd:  pds[0],
				pds: pds,
			},
			want: pds[:2], // only the first two, which are in IOMMU group 14
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getIommuSiblings(tt.args.pd, tt.args.pds, iommuGroupMap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getIommuSiblings() = %v, want %v", got, tt.want)
			}
		})
	}
}
