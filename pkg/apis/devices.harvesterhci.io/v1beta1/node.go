package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// a Node represents a k8s node and is used to reconcile objects on each node
type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

type NodeSpec struct{}

type NodeStatus struct {
	// GPUAddresses is used to track NVIDIA GPU addresses on the node
	// this can be used by validator to block pcidevice claims for addresses if node is
	// setup to be used for baremetal gpu container workloads
	GPUAddresses []string `json:"gpuAddresses,omitempty"`
}

const (
	NodeEnvVarName = "NODE_NAME"
	NodeKeyName    = "nodename"
)
