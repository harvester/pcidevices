package iommu

import (
	"reflect"
	"testing"
)

func TestGroupMapForPCIDevices(t *testing.T) {
	iommuGroups := []string{
		"/sys/kernel/iommu_groups/9/devices/0000:00:1c.0",
		"/sys/kernel/iommu_groups/9/devices/0000:00:1c.5",
		"/sys/kernel/iommu_groups/9/devices/0000:06:00.0",
		"/sys/kernel/iommu_groups/9/devices/0000:05:00.0",
		"/sys/kernel/iommu_groups/27/devices/0000:3e:04.2",
		"/sys/kernel/iommu_groups/27/devices/0000:3e:04.0",
		"/sys/kernel/iommu_groups/27/devices/0000:3e:04.3",
		"/sys/kernel/iommu_groups/27/devices/0000:3e:04.1",
	}
	tests := []struct {
		name string
		args []string
		want map[string]int
	}{
		{
			name: "Test GroupMapForPCIDevices",
			args: iommuGroups,
			want: map[string]int{
				"0000:00:1c.0": 9,
				"0000:00:1c.5": 9,
				"0000:06:00.0": 9,
				"0000:05:00.0": 9,
				"0000:3e:04.2": 27,
				"0000:3e:04.0": 27,
				"0000:3e:04.3": 27,
				"0000:3e:04.1": 27,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupMapForPCIDevices(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GroupMapForPCIDevices() = %v, want %v", got, tt.want)
			}
		})
	}
}
