package v1beta1

const (
	DeviceAllocationKey = "harvesterhci.io/deviceAllocationDetails"
	GPUPodsByNodeName   = "harvesterhci.io/gpu-pods-by-node"
)

type AllocationDetails struct {
	GPUs        map[string][]string `json:"gpus,omitempty"`
	HostDevices map[string][]string `json:"hostdevices,omitempty"`
}

var (
	GPUResourceName = "nvidia.com/gpu"
)
