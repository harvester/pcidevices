package v1beta1

import (
	"fmt"
	"strings"

	"regexp"

	"github.com/harvester/pcidevices/pkg/lspci"
	"github.com/jaypipes/pcidb"
	"github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/pci"
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
	Address           string   `json:"address"`
	VendorId          int      `json:"vendorId"`
	DeviceId          int      `json:"deviceId"`
	NodeName          string   `json:"nodeName"`
	Description       string   `json:"description"`
	KernelDriverInUse string   `json:"kernelDriverInUse,omitempty"`
	KernelModules     []string `json:"kernelModules"`
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

func description(pci *pcidb.PCIDB, vendorId string, deviceId string) string {
	vendor := pci.Vendors[vendorId]
	vendorCleaned := strings.ReplaceAll(strip(vendor.Name), " ", "")
	var product *pcidb.Product
	for _, product = range vendor.Products {
		// Example: 1c02 and 1C02 both represent the same device
		if strings.ToLower(product.ID) == strings.ToLower(deviceId) {
			// Found the product name
			productCleaned := strings.ReplaceAll(strip(product.Name), " ", "")
			return fmt.Sprintf("%s/%s", vendorCleaned, productCleaned)
		}
	}
	// If the pcidb doesn't have the deviceId, just show the deviceId
	return fmt.Sprintf("%s/%s", vendorCleaned, deviceId)
}

func (status *PCIDeviceStatus) Update(dev *pci.PCI, hostname string) {
	lspciOutput, err := lspci.GetLspciOuptut(dev.Addr)
	if err != nil {
		logrus.Error(err)
	}
	driver, err := lspci.ExtractCurrentPCIDriver(lspciOutput)
	if err != nil {
		logrus.Error(err)
		// Continue and update the object even if driver is not found
	}
	status.Address = dev.Addr
	status.VendorId = int(dev.Vendor)
	status.DeviceId = int(dev.Device)
	// Generate the Description field, this is used by KubeVirt to schedule the VM to the node
	pci, err := pcidb.New()
	if err != nil {
		logrus.Errorf("Error opening pcidb: %v", err)
	}
	vendorId := fmt.Sprintf("%x", dev.Vendor)
	deviceId := fmt.Sprintf("%x", dev.Device)
	status.Description = description(pci, vendorId, deviceId)

	status.KernelDriverInUse = driver
	status.NodeName = hostname

	modules, err := lspci.ExtractKernelModules(lspciOutput)
	if err != nil {
		logrus.Error(err)
		// Continue and update the object even if modules are not found
	}
	status.KernelModules = modules
}

type PCIDeviceSpec struct {
}

func PCIDeviceNameForHostname(dev *pci.PCI, hostname string) string {
	vendorName := strings.ToLower(
		strings.Split(dev.VendorName, " ")[0],
	)
	addrDNSsafe := strings.ReplaceAll(strings.ReplaceAll(dev.Addr, ":", ""), ".", "")
	return fmt.Sprintf(
		"%s-%s-%x-%x-%s",
		hostname,
		vendorName,
		dev.Vendor,
		dev.Device,
		addrDNSsafe,
	)
}

func NewPCIDeviceForHostname(dev *pci.PCI, hostname string) PCIDevice {
	name := PCIDeviceNameForHostname(dev, hostname)
	pciDevice := PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: PCIDeviceStatus{
			Address:     dev.Addr,
			VendorId:    int(dev.Vendor), // upcasting a uint16 to an int is safe
			DeviceId:    int(dev.Device),
			NodeName:    hostname,
			Description: dev.DeviceName,
		},
	}
	return pciDevice
}
