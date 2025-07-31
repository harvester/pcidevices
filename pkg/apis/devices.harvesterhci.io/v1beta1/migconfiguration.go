package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigConfiguration represents the instructions for managing MIG profiles on supported GPUs
type MigConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigConfigurationSpec   `json:"spec,omitempty"`
	Status MigConfigurationStatus `json:"status,omitempty"`
}

type MigConfigurationSpec struct {
	Enabled     bool                `json:"enabled"`
	GPUAddress  string              `json:"gpuAddress"`
	NodeName    string              `json:"nodeName"`
	ProfileSpec []MigProfileRequest `json:"profileSpec,omitempty"`
}

type MigConfigurationStatus struct {
	ProfileStatus []MigProfileStatus     `json:"profileStatus,omitempty"`
	Status        MIGConfigurationStatus `json:"status"`
}

type MigProfiles struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// MigProfileStatus contains information of created MIG profiles
type MigProfileStatus struct {
	MigProfiles `json:",inline"`
	// Available indicates how many more instances of a particular profile are possible
	Available int `json:"available"`
	// Total indicates max count of instances of a particular profile possible
	Total int `json:"total"`
	// ID tracks the Instance ID of generated MIG devices
	// and is needed for deletion of MIG profiles
	VGPUID []string `json:"vGPUID,omitempty"`
}

// MigProfileRequest contains information about MIG profile creation
type MigProfileRequest struct {
	MigProfiles `json:",inline"`
	Requested   int `json:"requested"`
}

type MIGConfigurationStatus string

const (
	MIGConfigurationSynced    MIGConfigurationStatus = "synced"
	MIGConfigurationOutOfSync MIGConfigurationStatus = "out-of-sync"
	MIGConfigurationDisabled  MIGConfigurationStatus = "disabled"
)
