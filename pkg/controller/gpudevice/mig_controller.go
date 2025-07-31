package gpudevice

import (
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

const (
	defaultMIGRequeuePeriod = 300 * time.Second
)

// OnMIGChange will reconcile MIG Instances based on definition of MIGConfiguration object
func (h *Handler) OnMIGChange(name string, mig *v1beta1.MigConfiguration) (*v1beta1.MigConfiguration, error) {
	if mig == nil || mig.DeletionTimestamp != nil || mig.Spec.NodeName != h.nodeName {
		return mig, nil
	}

	status, err := gpuhelper.GenerateMIGConfigurationStatus(h.executor, mig.Spec.GPUAddress)
	if err != nil {
		return mig, fmt.Errorf("error generating MIG configuration status for device %s: %w", mig.Name, err)
	}

	migCopy := mig.DeepCopy()
	// no devices are setup so we need to enable the devices
	if mig.Spec.Enabled && instanceCount(status) == 0 {
		// need to check based on status if any further action is needed
		// fetch MIG instance status
		logrus.Debugf("setting up mig instances for device %s", mig.Name)
		err = gpuhelper.EnableMIGProfiles(h.executor, mig)
		if err != nil {
			return mig, fmt.Errorf("error setting up MIG instances for device %s: %w", mig.Name, err)
		}
		// fetch MIG instance status
		status, err = gpuhelper.GenerateMIGConfigurationStatus(h.executor, mig.Spec.GPUAddress)
		if err != nil {
			return mig, fmt.Errorf("error generating MIG configuration status for device %s: %w", mig.Name, err)
		}
		migCopy.Status = *status
		return h.migConfigurationController.UpdateStatus(migCopy)
	}

	// reconcile current and existing status
	if mig.Spec.Enabled {
		// existing and defined instances match no further action needed
		if reflect.DeepEqual(mig.Status.ProfileStatus, status.ProfileStatus) {
			migCopy.Status.Status = v1beta1.MIGConfigurationSynced
		} else {
			migCopy.Status.Status = v1beta1.MIGConfigurationOutOfSync
		}

		if mig.Status.Status != migCopy.Status.Status {
			return h.migConfigurationController.UpdateStatus(migCopy)
		}
	}

	// mig configuration is disabled but instance count is not 0
	// trigger deletion of instances
	if !mig.Spec.Enabled && instanceCount(status) != 0 {
		// disable MIG profiles
		logrus.Debugf("disabling MIG instances for device %s", mig.Name)
		err = gpuhelper.DisableMIGProfiles(h.executor, mig)
		if err != nil {
			return mig, err
		}

		// update status of object
		status, err = gpuhelper.GenerateMIGConfigurationStatus(h.executor, mig.Spec.GPUAddress)
		if err != nil {
			return mig, fmt.Errorf("error generating MIG configuration status for device %s: %w", mig.Name, err)
		}
		migCopy.Status = *status
		migCopy.Status.Status = v1beta1.MIGConfigurationDisabled
		return h.migConfigurationController.UpdateStatus(migCopy)
	}

	// requeue MIGConfiguration every 5 mins to ensure host status matches CRD status
	h.migConfigurationController.EnqueueAfter(name, defaultMIGRequeuePeriod)
	return mig, nil
}

// instanceCount returns number of instances configured based on discover status
func instanceCount(discoveredStatus *v1beta1.MigConfigurationStatus) int {
	var discoveredInstances int
	for _, v := range discoveredStatus.ProfileStatus {
		discoveredInstances = discoveredInstances + len(v.VGPUID)
	}
	return discoveredInstances
}
