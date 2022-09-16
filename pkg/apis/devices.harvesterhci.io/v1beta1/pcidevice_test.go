package v1beta1

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jaypipes/pcidb"
	"github.com/u-root/u-root/pkg/pci"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewPCIDeviceForName(t *testing.T) {
	type args struct {
		dev      *pci.PCI
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
				dev: &pci.PCI{
					VendorName: "Intel Corporation",
					Vendor:     0x8086,
					Device:     0x1521,
					Addr:       "00:1f.6",
				},
				hostname: "deepgreen",
			},
			want: PCIDevice{
				ObjectMeta: v1.ObjectMeta{
					Name: "deepgreen-intel-8086-1521-001f6",
				},
				Status: PCIDeviceStatus{
					NodeName: "deepgreen",
					VendorId: 0x8086,
					DeviceId: 0x1521,
					Address:  "00:1f.6",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPCIDeviceForHostname(tt.args.dev, tt.args.hostname)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nNewPCIDeviceForHostname() = %v,\nwant %v", got, tt.want)
			}
		})
	}
}

func TestDescriptionForVendorDevice(t *testing.T) {
	pci, err := pcidb.New()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Loop through all Vendors and all Products
	file, err := os.OpenFile("/home/tobi/descriptions.txt", os.O_RDWR, 0777)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer file.Close()
	for _, v := range pci.Vendors {
		vendor := v
		for _, product := range vendor.Products {
			file.WriteString(
				fmt.Sprintf(
					"%s %s: %s\n",
					vendor.ID, product.ID,
					description(pci, vendor.ID, product.ID),
				),
			)
		}
	}
	// end debug loop

	type args struct {
		vendorId string
		deviceId string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "NVIDIA A100",
			args: args{
				vendorId: "10de",
				deviceId: "20b0",
			},
			want: "nvidia.com/GA100A100SXM440GB",
		},
		{
			name: "NVIDIA GeForce GTX 1060",
			args: args{
				vendorId: "10de",
				deviceId: "1c02",
			},
			want: "nvidia.com/GP106GeForceGTX10603GB",
		},
		{
			name: "AMD Radeon X850",
			args: args{
				vendorId: "1002",
				deviceId: "4b49",
			},
			want: "amd.com/R481RadeonX850XTAGP",
		},
		{
			name: "Intel I350 Gigabit Network Connection",
			args: args{
				vendorId: "8086",
				deviceId: "1521",
			},
			want: "intel.com/I350GigabitNetworkConnection",
		},
		{
			name: "Mellanox MT2892",
			args: args{
				vendorId: "15b3",
				deviceId: "0212",
			},
			want: "mellanox.com/MT2892FamilyConnectX6DxFlashRecovery",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := description(
				pci,
				tt.args.vendorId,
				tt.args.deviceId,
			)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\ndescription() = %v,\nwant %v", got, tt.want)
			}
		})
	}
}
