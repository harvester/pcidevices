package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

type USBDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status USBDeviceStatus `json:"status,omitempty"`
}

type USBDeviceStatus struct {
	VendorID     string `json:"vendorID"`
	ProductID    string `json:"productID"`
	NodeName     string `json:"nodeName"`
	ResourceName string `json:"resourceName"`
	DevicePath   string `json:"devicePath"`
}
