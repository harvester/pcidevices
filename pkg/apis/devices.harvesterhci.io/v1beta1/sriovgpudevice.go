package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// a SRIOVGPUDevice represents an srio-v capable gpu on a node in the cluster
type SRIOVGPUDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SRIOVGPUDeviceSpec   `json:"spec,omitempty"`
	Status SRIOVGPUDeviceStatus `json:"status,omitempty"`
}

type SRIOVGPUDeviceSpec struct {
	Address  string `json:"address"`
	NodeName string `json:"nodeName"`
	Enabled  bool   `json:"enabled"`
}

type SRIOVGPUDeviceStatus struct {
	VFAddresses []string `json:"vfAddresses,omitempty"`
	VGPUDevices []string `json:"vGPUDevices,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// a VGPUDevice represents a configure vGPU on the node in cluster
type VGPUDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VGPUDeviceSpec   `json:"spec,omitempty"`
	Status VGPUDeviceStatus `json:"status,omitempty"`
}

type VGPUDeviceSpec struct {
	VGPUTypeName           string `json:"vGPUTypeName"`
	Address                string `json:"address"`
	Enabled                bool   `json:"enabled"`
	NodeName               string `json:"nodeName"`
	ParentGPUDeviceAddress string `json:"parentGPUDeviceAddress"`
}

type VGPUDeviceStatus struct {
	VGPUStatus             VGPUStatus        `json:"vGPUStatus,omitempty"`
	UUID                   string            `json:"uuid,omitempty"`
	ConfiguredVGPUTypeName string            `json:"configureVGPUTypeName,omitempty"`
	AvailableTypes         map[string]string `json:"availableTypes,omitempty"` // reconciles against mdev_supported_types and populates name: mdevType for types which are possible
}

type VGPUStatus string

const (
	HarvesterVGPUType                    = "vgpu.harvesterhci.io/type"
	VGPUEnabled               VGPUStatus = "vGPUConfigured"
	VGPUDisabled              VGPUStatus = ""
	SysDevRoot                           = "/sys/bus/pci/devices/"
	MdevRoot                             = "/sys/bus/mdev/devices/"
	MdevBusClassRoot                     = "/sys/class/mdev_bus/"
	MdevSupportTypesDir                  = "mdev_supported_types"
	ParentSRIOVGPUDeviceLabel            = "harvesterhci.io/parentSRIOVGPUDevice"
	DefaultNamespace                     = "harvester-system"
	NvidiaDriverLabel                    = "app=nvidia-driver-daemonset"
	NvidiaDriverNeededKey                = "sriovgpu.harvesterhci.io/driver-needed"
)
