package v1beta1

import (
	"fmt"
	"strings"

	"regexp"

	"github.com/jaypipes/ghw/pkg/pci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// PCIDevice is the Schema for the pcidevices API
type PCIDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCIDeviceSpec   `json:"spec,omitempty"`
	Status PCIDeviceStatus `json:"status,omitempty"`
}

// PCIDeviceStatus defines the observed state of PCIDevice
type PCIDeviceStatus struct {
	Address           string `json:"address"`
	VendorId          string `json:"vendorId"`
	DeviceId          string `json:"deviceId"`
	NodeName          string `json:"nodeName"`
	Description       string `json:"description"`
	KernelDriverInUse string `json:"kernelDriverInUse,omitempty"`
}

func strip(s string) string {
	// Make a Regex to say we only want
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		fmt.Printf("%v", err)
	}
	processedString := reg.ReplaceAllString(s, "")
	return processedString
}

func extractVendorNameFromBrackets(vendorName string) string {
	// Make a Regex to say we only want
	reg, err := regexp.Compile(`\[([^\]]+)\]`)
	if err != nil {
		fmt.Printf("%v", err)
	}
	matches := reg.FindStringSubmatch(vendorName)
	preSlash := strings.Split(matches[1], "/")[0]
	return strip(preSlash)
}

func description(dev *pci.Device) string {
	var vendorBase string
	// if vendor name has a '[name]', then use that
	if strings.Contains(dev.Vendor.Name, "[") {
		vendorBase = extractVendorNameFromBrackets(dev.Vendor.Name)
	} else {
		vendorBase = strip(strings.Split(dev.Vendor.Name, " ")[0])
	}
	vendorCleaned := strings.ToLower(
		strings.ReplaceAll(vendorBase, " ", ""),
	) + ".com"
	if dev.Product.Name != "" {
		productCleaned := strings.ReplaceAll(strip(dev.Product.Name), " ", "")
		return fmt.Sprintf("%s/%s", vendorCleaned, productCleaned)
	}
	// If the pcidb doesn't have the deviceId, just show the deviceId
	return fmt.Sprintf("%s/%s", vendorCleaned, dev.Product.ID)
}

func (status *PCIDeviceStatus) Update(dev *pci.Device, hostname string) {
	status.Address = dev.Address
	status.VendorId = dev.Vendor.ID
	status.DeviceId = dev.Product.ID
	// Generate the Description field, this is used by KubeVirt to schedule the VM to the node
	status.Description = description(dev)

	status.KernelDriverInUse = dev.Driver
	status.NodeName = hostname
}

type PCIDeviceSpec struct {
}

func PCIDeviceNameForHostname(dev *pci.Device, hostname string) string {
	addrDNSsafe := strings.ReplaceAll(strings.ReplaceAll(dev.Address, ":", ""), ".", "")
	return fmt.Sprintf(
		"%s-%s-%s-%s",
		hostname,
		dev.Vendor.ID,
		dev.Product.ID,
		addrDNSsafe,
	)
}

func NewPCIDeviceForHostname(dev *pci.Device, hostname string) PCIDevice {
	name := PCIDeviceNameForHostname(dev, hostname)
	pciDevice := PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: PCIDeviceStatus{
			Address:     dev.Address,
			VendorId:    dev.Vendor.ID,
			DeviceId:    dev.Product.ID,
			NodeName:    hostname,
			Description: dev.Product.Name,
		},
	}
	return pciDevice
}
