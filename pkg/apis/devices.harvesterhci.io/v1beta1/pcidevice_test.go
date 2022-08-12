package v1beta1

import (
	"reflect"
	"testing"

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
