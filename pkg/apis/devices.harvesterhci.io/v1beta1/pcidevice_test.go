package v1beta1

import (
	"reflect"
	"testing"

	"github.com/jaypipes/ghw/pkg/pci"
	"github.com/jaypipes/pcidb"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewPCIDeviceForName(t *testing.T) {
	type args struct {
		dev      *pci.Device
		hostname string
	}
	tests := []struct {
		name string
		args args
		want PCIDevice
	}{
		{
			name: "Metadata is present",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "8086",
						Name: "Intel Corporation",
					},
					Product: &pcidb.Product{
						ID:   "1521",
						Name: "I350 Gigabit Network Connection",
					},
					Class: &pcidb.Class{
						ID:   "02",
						Name: "Network controller",
					},
					Subclass: &pcidb.Subclass{
						ID:   "00",
						Name: "Ethernet controller",
					},
					Driver: "fake",
				},
				hostname: "deepgreen",
			},
			want: PCIDevice{
				ObjectMeta: v1.ObjectMeta{
					Name: "deepgreen-001f6",
					Annotations: map[string]string{
						PciDeviceDriver: "fake",
					},
				},
				Status: PCIDeviceStatus{
					NodeName:          "deepgreen",
					VendorID:          "8086",
					DeviceID:          "1521",
					ClassID:           "0200",
					ResourceName:      "intel.com/I350_GIGABIT_NETWORK_CONNECTION",
					Description:       "Ethernet controller: Intel Corporation I350 Gigabit Network Connection",
					Address:           "00:1f.6",
					KernelDriverInUse: "fake",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPCIDeviceForHostname(tt.args.dev, tt.args.hostname)
			t.Log(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nNewPCIDeviceForHostname() = %v,\nwant %v", got, tt.want)
			}
		})
	}
}

func TestDescriptionForVendorDevice(t *testing.T) {
	type args struct {
		dev *pci.Device
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "NVIDIA A100",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "10de",
						Name: "NVIDIA Corporation",
					},
					Product: &pcidb.Product{
						ID:   "20b0",
						Name: "GA100 [A100 SXM4 40GB]",
					},
				},
			},
			want: "nvidia.com/GA100_A100_SXM4_40GB",
		},
		{
			name: "NVIDIA GeForce GTX 1060",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "10de",
						Name: "NVIDIA Corporation",
					},
					Product: &pcidb.Product{
						ID:   "1c02",
						Name: "GP106 [GeForce GTX 1060 3GB]",
					},
				},
			},
			want: "nvidia.com/GP106_GEFORCE_GTX_1060_3GB",
		},
		{
			name: "AMD Radeon X850",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "1002",
						Name: "Advanced Micro Devices, Inc. [AMD/ATI]",
					},
					Product: &pcidb.Product{
						ID:   "4b49",
						Name: "R481 [Radeon X850 XT AGP]",
					},
				},
			},
			want: "amd.com/R481_RADEON_X850_XT_AGP",
		},
		{
			name: "Intel I350 Gigabit Network Connection",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "8086",
						Name: "Intel Corporation",
					},
					Product: &pcidb.Product{
						ID:   "1521",
						Name: "I350 Gigabit Network Connection",
					},
				},
			},
			want: "intel.com/I350_GIGABIT_NETWORK_CONNECTION",
		},
		{
			name: "Mellanox MT2892",
			args: args{
				dev: &pci.Device{
					Address: "00:1f.6",
					Vendor: &pcidb.Vendor{
						ID:   "15b3",
						Name: "Mellanox Technologies",
					},
					Product: &pcidb.Product{
						ID:   "0212",
						Name: "MT2892 Family [ConnectX-6 Dx Flash Recovery]",
					},
				},
			},
			want: "mellanox.com/MT2892_FAMILY_CONNECTX6_DX_FLASH_RECOVERY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resourceName(
				tt.args.dev,
			)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\ndescription() = %v,\nwant %v", got, tt.want)
			}
		})
	}
}
