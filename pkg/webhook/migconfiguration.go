package webhook

import (
	"fmt"
	"reflect"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"

	"github.com/harvester/harvester/pkg/webhook/types"
)

const (
	MaxMIGInstanceCount = 7 //limit enforced by NVIDIA
)

type migConfigurationValidator struct {
	types.DefaultValidator
	vGPUCache v1beta1.VGPUDeviceCache
}

func NewMIGConfigurationValidator(vGPUCache v1beta1.VGPUDeviceCache) types.Validator {
	return &migConfigurationValidator{
		vGPUCache: vGPUCache,
	}
}

func (m *migConfigurationValidator) Resource() types.Resource {
	return types.Resource{
		Names:      []string{"migconfigurations"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   devicesv1beta1.SchemeGroupVersion.Group,
		APIVersion: devicesv1beta1.SchemeGroupVersion.Version,
		ObjectType: &devicesv1beta1.MigConfiguration{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Update,
		},
	}
}

func (m *migConfigurationValidator) Update(_ *types.Request, oldObj runtime.Object, newObj runtime.Object) error {
	oldMigConfiguration := oldObj.(*devicesv1beta1.MigConfiguration)
	newMigConfiguration := newObj.(*devicesv1beta1.MigConfiguration)

	// specifying more than 7 MIG instances in request gets rejected
	count := instanceCount(newMigConfiguration)
	if count > MaxMIGInstanceCount {
		return fmt.Errorf("MIGConfiguration %s Invalid: Cannot have more than %d instances defined, found %d", newMigConfiguration.Name, MaxMIGInstanceCount, count)
	}

	// attempt to configure instances while configuration is already synced/out-of-sync
	// gets rejected
	if oldMigConfiguration.Status.Status == devicesv1beta1.MIGConfigurationOutOfSync || oldMigConfiguration.Status.Status == devicesv1beta1.MIGConfigurationSynced {
		if !reflect.DeepEqual(oldMigConfiguration.Spec.ProfileSpec, newMigConfiguration.Spec.ProfileSpec) {
			return fmt.Errorf("MIGConfiguration %s cannot be modified unless configuration is disabled", oldMigConfiguration.Name)
		}
	}

	// ensure request count for profile cannot exceed total count when enabling a MIG configuration
	if !oldMigConfiguration.Spec.Enabled && newMigConfiguration.Spec.Enabled {
		return validRequestedCount(newMigConfiguration)
	}

	// if MIGConfiguration is being disabled, verify there are no vGPU devices
	// which are enabled as they are going to be using the MIG Instances
	if oldMigConfiguration.Spec.Enabled && !newMigConfiguration.Spec.Enabled {
		return m.verifyInUsevGPU(newMigConfiguration)
	}
	return nil
}

func instanceCount(obj *devicesv1beta1.MigConfiguration) int {
	requestedInstanceCount := 0
	for _, v := range obj.Spec.ProfileSpec {
		requestedInstanceCount = requestedInstanceCount + v.Requested
	}
	return requestedInstanceCount
}

func validRequestedCount(obj *devicesv1beta1.MigConfiguration) error {
	// instanceCountMap contains details of available instances for a specific profile
	// instanceCountMap[profileID]= AvailableCount
	instanceCountMap := make(map[int]int)
	for _, v := range obj.Status.ProfileStatus {
		instanceCountMap[v.ID] = v.Available
	}

	for _, v := range obj.Spec.ProfileSpec {
		availableCount := instanceCountMap[v.ID]
		if v.Requested > availableCount {
			return fmt.Errorf("MIGConfiguration %s cannot has requested count %d for profile %s, which exceeds available count %d", obj.Name, v.Requested, v.Name, availableCount)
		}
	}
	return nil
}

func (m *migConfigurationValidator) verifyInUsevGPU(obj *devicesv1beta1.MigConfiguration) error {
	vGPUset := map[string]string{
		devicesv1beta1.ParentSRIOVGPUDeviceLabel: obj.Name,
	}
	labelSelector := labels.SelectorFromSet(vGPUset)

	vGPUList, err := m.vGPUCache.List(labelSelector)
	if err != nil {
		return fmt.Errorf("error looking up vGPU from cache: %w", err)
	}

	for _, v := range vGPUList {
		if v.Spec.Enabled {
			return fmt.Errorf("cannot disable MIGConfiguration %s as vGPU %s is enabled", obj.Name, v.Name)
		}
	}
	return nil
}
