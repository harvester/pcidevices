package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// a SRIOVNetworkDevice represents an srio-v capable network interface on a node in the cluster
type SRIOVNetworkDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SRIOVNetworkDeviceSpec   `json:"spec,omitempty"`
	Status SRIOVNetworkDeviceStatus `json:"status,omitempty"`
}

type SRIOVNetworkDeviceSpec struct {
	Address  string `json:"address"`
	NodeName string `json:"nodeName"`
	NumVFs   int    `json:"numVFs"`
}

type SRIOVNetworkDeviceStatus struct {
	VFAddresses  []string `json:"vfAddresses,omitempty"`
	VFPCIDevices []string `json:"vfPCIDevices,omitempty"`
	Status       string   `json:"status"`
}

const (
	DeviceDisabled           = "sriovNetworkDeviceDisabled"
	DeviceEnabled            = "sriovNetworkDeviceEnabled"
	ParentSRIOVNetworkDevice = "harvesterhci.io/parent-sriov-network-device"
	SRIOVFromVF              = "sriov-dev-from-vf"
)
