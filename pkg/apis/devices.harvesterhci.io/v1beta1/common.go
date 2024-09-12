package v1beta1

const (
	DeviceAllocationKey = "harvesterhci.io/deviceAllocationDetails"
)

type AllocationDetails struct {
	GPUs        map[string][]string `json:"gpus,omitempty"`
	HostDevices map[string][]string `json:"hostdevices,omitempty"`
}
