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
