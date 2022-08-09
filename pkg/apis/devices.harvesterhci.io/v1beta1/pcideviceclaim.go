package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// a PCIDeviceClaim is used to reserve a PCI Device for a single
type PCIDeviceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCIDeviceClaimSpec   `json:"spec,omitempty"`
	Status PCIDeviceClaimStatus `json:"status,omitempty"`
}

type PCIDeviceClaimSpec struct {
	Address  string `json:"address"`
	NodeName string `json:"nodeName"`
	UserName string `json:"userName"`
}

type PCIDeviceClaimStatus struct {
	KernelDriverToUnbind string `json:"kernelDriverToUnbind"`
	PassthroughEnabled   bool   `json:"passthroughEnabled"`
}
