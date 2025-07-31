package webhook

import (
	"github.com/stretchr/testify/require"

	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/pcidevices/pkg/util/fakeclients"
)

var (
	migConfig1 = &devicesv1beta1.MigConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-000004000",
		},
		Spec: devicesv1beta1.MigConfigurationSpec{
			Enabled:    false,
			NodeName:   "node1",
			GPUAddress: "0000:04:00.0",
			ProfileSpec: []devicesv1beta1.MigProfileRequest{
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb",
						ID:   19,
					},
					Requested: 0,
				},
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb+me",
						ID:   20,
					},
					Requested: 0,
				},
			},
		},
		Status: devicesv1beta1.MigConfigurationStatus{
			ProfileStatus: []devicesv1beta1.MigProfileStatus{
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb",
						ID:   19,
					},
					Available: 7,
					Total:     7,
				},
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb+me",
						ID:   20,
					},
					Available: 2,
					Total:     2,
				},
			},
		},
	}

	migConfig2 = &devicesv1beta1.MigConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1-000005000",
		},
		Spec: devicesv1beta1.MigConfigurationSpec{
			Enabled:    true,
			NodeName:   "node1",
			GPUAddress: "0000:05:00.0",
			ProfileSpec: []devicesv1beta1.MigProfileRequest{
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb",
						ID:   19,
					},
					Requested: 0,
				},
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb+me",
						ID:   20,
					},
					Requested: 0,
				},
			},
		},
		Status: devicesv1beta1.MigConfigurationStatus{
			ProfileStatus: []devicesv1beta1.MigProfileStatus{
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb",
						ID:   19,
					},
					Available: 7,
					Total:     7,
				},
				{
					MigProfiles: devicesv1beta1.MigProfiles{
						Name: "MIG 1g.5gb+me",
						ID:   20,
					},
					Available: 1,
					Total:     1,
				},
			},
		},
	}

	vGPUList = &devicesv1beta1.VGPUDeviceList{
		Items: []devicesv1beta1.VGPUDevice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1-vgpu1",
					Labels: map[string]string{
						devicesv1beta1.ParentSRIOVGPUDeviceLabel: migConfig1.Name,
					},
				},
				Spec: devicesv1beta1.VGPUDeviceSpec{
					Enabled: true,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1-vgpu2",
					Labels: map[string]string{
						devicesv1beta1.ParentSRIOVGPUDeviceLabel: migConfig1.Name,
					},
				},
				Spec: devicesv1beta1.VGPUDeviceSpec{
					Enabled: false,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1-vgpu3",
					Labels: map[string]string{
						devicesv1beta1.ParentSRIOVGPUDeviceLabel: migConfig2.Name,
					},
				},
				Spec: devicesv1beta1.VGPUDeviceSpec{
					Enabled: false,
				},
			},
		},
	}

	fakeClient   = fake.NewSimpleClientset(vGPUList)
	migValidator = NewMIGConfigurationValidator(fakeclients.VGPUDeviceCache(fakeClient.DevicesV1beta1().VGPUDevices))
)

func Test_ValidateMaxMIGProfiles(t *testing.T) {
	migConfig1Copy := migConfig1.DeepCopy()
	migConfig1Copy.Spec.ProfileSpec[0].Requested = 6
	migConfig1Copy.Spec.ProfileSpec[1].Requested = 1
	migConfig1Copy.Spec.Enabled = true
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig1, migConfig1Copy)
	assert.NoError(err, "expected no error during migconfiguration validation")
}

// Test_ExceedMaxMIGProfiles checks if total number of MIG instances requested exceeds 7
// but each individual instance type is within the limit of available count for that
// particular instance
func Test_ExceedMaxMIGProfiles(t *testing.T) {
	migConfig1Copy := migConfig1.DeepCopy()
	migConfig1Copy.Spec.ProfileSpec[0].Requested = 6
	migConfig1Copy.Spec.ProfileSpec[1].Requested = 2
	migConfig1Copy.Spec.Enabled = true
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig1, migConfig1Copy)
	assert.Error(err, "expected error as max mig profiles have been exceeded")
}

// Test_ExceedIndividualMigProfiles checks if request exceeds individual available
// count for the particular MIG instance type
func Test_ExceedIndividualMigProfiles(t *testing.T) {
	migConfig1Copy := migConfig1.DeepCopy()
	migConfig1Copy.Spec.ProfileSpec[1].Requested = 3
	migConfig1Copy.Spec.Enabled = true
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig1, migConfig1Copy)
	assert.Error(err, "expected error as max instances of a particular MIG profile have been exceeded")
}

// Test_ExceedIndividualMigProfilesWithDisabledGPU checks a condition where max mig instance profiles for a particular instance have been exceeded
// but no error is reported as MIGConfiguration is not enabled so
// no actual work is expected from executor
func Test_ExceedIndividualMigProfilesWithDisabledGPU(t *testing.T) {
	migConfig1Copy := migConfig1.DeepCopy()
	migConfig1Copy.Spec.ProfileSpec[0].Requested = 8
	migConfig1Copy.Spec.Enabled = false
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig1, migConfig1Copy)
	assert.Error(err, "expected error as max instances of a particular MIG profile have been exceeded")
}

// Test_DisableMigConfigurationWithInUseVGPU checks if MIGConfiguration managed instance may be in use with a vGPU
// and blocks the attempt to disable MIG Configuration
func Test_DisableMigConfigurationWithInUseVGPU(t *testing.T) {
	migConfig1Copy := migConfig1.DeepCopy()
	migConfig1Copy.Spec.Enabled = true
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig1Copy, migConfig1)
	assert.Error(err, "expected error while disable MIG configuration as there is a VGPUDevice using the MIG profile")
}

// Test_DisableMigConfigurationWithoutInUseVGPU checks if MIGConfiguration managed instance may be in use with a vGPU
// and allows the attempt to disable MIG Configuration since no associated vGPU is enabled
func Test_DisableMigConfigurationWithoutInUseVGPU(t *testing.T) {
	migConfig2Copy := migConfig2.DeepCopy()
	migConfig2Copy.Spec.Enabled = false
	assert := require.New(t)
	err := migValidator.Update(nil, migConfig2, migConfig2Copy)
	assert.NoError(err, "expected no error while disabling MIGConfiguration")
}
