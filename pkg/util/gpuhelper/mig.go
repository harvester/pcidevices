package gpuhelper

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/executor"
)

const (
	MigPrefix = "MIG"
	NA        = "[N/A]"
)

// IsMigConfigurationNeeded checks if MIG devices are supported by underlying GPU device
// this is used as a precursor to check if MIGConfiguration object needs to be created for the GPU
func IsMigConfigurationNeeded(ex executor.Executor, deviceAddress string) (bool, error) {
	migCheckCommand := generateMigStatusCheckCommand(deviceAddress)
	result, err := ex.Run(migCheckCommand, nil)
	if err != nil {
		return false, fmt.Errorf("error executing %s: %w", migCheckCommand, err)
	}
	return isMigSupported(string(result)), nil
}

// GenerateMIGConfiguration will generate a MIGConfiguration object which can be used to
// subsequently configure the MIG profiles on underlying GPUs
func GenerateMIGConfiguration(ex executor.Executor, deviceAddress string, deviceName string) (*v1beta1.MigConfiguration, error) {
	profileCommand := generateListMIGProfileCommand(deviceAddress)
	profileResult, err := ex.Run(profileCommand, nil)
	if err != nil {
		return nil, fmt.Errorf("error executing %s: %w", profileCommand, err)
	}

	profileInfo, err := generateProfileSpec(string(profileResult))
	if err != nil {
		return nil, fmt.Errorf("error generating migconfiguration profile info: %w", err)
	}

	migConfig := &v1beta1.MigConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: deviceName,
		},
		Spec: v1beta1.MigConfigurationSpec{
			Enabled:     false,
			GPUAddress:  deviceAddress,
			ProfileSpec: profileInfo,
		},
	}

	return migConfig, nil
}

// GenerateMIGConfigurationStatus will query and identify current instance information from GPU
// this is used to update the MIGConfiguration object after the profiles have been setup
func GenerateMIGConfigurationStatus(ex executor.Executor, deviceAddress string) (*v1beta1.MigConfigurationStatus, error) {
	instanceInfoCommand := generateListMigInstanceInfoCommand(deviceAddress)
	instanceResult, err := ex.Run(instanceInfoCommand, nil)
	if err != nil {
		return nil, fmt.Errorf("error executing %s: %w", instanceInfoCommand, err)
	}

	profileCommand := generateListMIGProfileCommand(deviceAddress)
	profileResult, err := ex.Run(profileCommand, nil)
	if err != nil {
		return nil, fmt.Errorf("error executing %s: %w", profileCommand, err)
	}

	profileStatus, err := GenerateProfileStatus(string(profileResult), string(instanceResult))
	if err != nil {
		return nil, fmt.Errorf("error generating profile status: %v", err)
	}

	return &v1beta1.MigConfigurationStatus{
		ProfileStatus: profileStatus,
	}, nil
}

// EnableMIGProfiles interacts with the GPU driver and sets up the MIG instances
// which can be further used with vGPU's and Harvester
func EnableMIGProfiles(ex executor.Executor, migConfig *v1beta1.MigConfiguration) error {
	var impactedProfiles []int
	// requestMap is a map of profileID: requested count
	// we build this map while looping over profileSpec to
	// make it easier subsequently to find requested count
	// once the profileID is sorted
	requestMap := make(map[int]int)
	for _, v := range migConfig.Spec.ProfileSpec {
		if v.Requested > 0 {
			impactedProfiles = append(impactedProfiles, v.ID)
			requestMap[v.ID] = v.Requested
		}
	}

	// nvidia-smi assigns the larger profiles a smaller ID
	// we will sort impactedProfiles to ensure larger profiles are created first
	// to avoid potential placement issues
	sortedProfiles := sort.IntSlice(impactedProfiles)

	for _, profile := range sortedProfiles {
		for i := 0; i < requestMap[profile]; i++ {
			err := CreateMIGInstance(ex, migConfig.Spec.GPUAddress, profile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func generateProfileSpec(input string) ([]v1beta1.MigProfileRequest, error) {
	info, err := parseMigInfo(input)
	if err != nil {
		return nil, fmt.Errorf("error parsing mig info: %w", err)
	}
	profileRequests := make([]v1beta1.MigProfileRequest, 0, len(info))
	// generate mig profile info from output which looks like a sequence of lines
	// |   0  MIG 1g.5gb+me     20     1/1        4.75       No     14     1     0   |
	re := regexp.MustCompile(`\s+`)
	for _, v := range info {
		v = strings.ReplaceAll(v, "|", "")
		v = strings.TrimSpace(v)
		cleaned := re.ReplaceAllString(v, ",")
		profile, _, _, err := generateProfile(cleaned)
		if err != nil {
			return nil, fmt.Errorf("error calling generateProfile in GenerateProfileSpec: %w", err)
		}
		pr := v1beta1.MigProfileRequest{
			MigProfiles: profile,
			Requested:   0,
		}
		profileRequests = append(profileRequests, pr)
	}
	return profileRequests, nil
}

func parseMigInfo(input string) ([]string, error) {
	var info []string
	scanner := bufio.NewScanner(bytes.NewBuffer([]byte(input)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, MigPrefix) {
			info = append(info, line)
		} // Println will add back the final '\n'
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return info, nil
}

// generateProfile will parse output of `nvidia-smi mig -lgip` and generate a MigProfiles struct
func generateProfile(profileInfo string) (v1beta1.MigProfiles, string, string, error) {
	var err error
	profile := v1beta1.MigProfiles{}
	elements := strings.Split(profileInfo, ",")
	// format of string will be of form
	// [0 MIG 1g.5gb 19 7/7 4.75 No 14 0 0]
	profile.Name = fmt.Sprintf("%s %s", elements[1], elements[2])
	profile.ID, err = strconv.Atoi(elements[3])
	if err != nil {
		return profile, "", "", fmt.Errorf("error converting profile id %s to string: %w", elements[3], err)
	}
	capacity := strings.Split(elements[4], "/")
	// return profile, available profile capacity, total profile capacity
	return profile, capacity[0], capacity[1], nil
}

// generateMIGInstanceInfo will generate a map of MIG profile and associated instance IDs
// this is subsequently used to populate MigProfileStatus
func generateMIGInstanceInfo(instanceInfo string) (map[int][]string, error) {
	info, err := parseMigInfo(instanceInfo)
	if err != nil {
		return nil, fmt.Errorf("error parsing instance info: %w", err)
	}
	// profileStringMap is of type map[string]string
	// to allow easier sorting based on profile ID we will convert to
	// map[int][]string
	profileStringMap := generateProfileInstanceMap(info)
	profileIntMap := make(map[int][]string)
	for k, v := range profileStringMap {
		intK, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("error converting %s to int %w", k, err)
		}
		profileIntMap[intK] = v
	}
	return profileIntMap, nil
}

func GenerateProfileStatus(profileInfo string, instanceInfo string) ([]v1beta1.MigProfileStatus, error) {

	info, err := parseMigInfo(profileInfo)
	if err != nil {
		return nil, fmt.Errorf("error parsing mig info: %w", err)
	}
	profileStatuses := make([]v1beta1.MigProfileStatus, 0, len(info))
	// generate mig profile info from output which looks like a sequence of lines
	// |   0  MIG 1g.5gb+me     20     1/1        4.75       No     14     1     0   |
	re := regexp.MustCompile(`\s+`)
	for _, v := range info {
		v = strings.ReplaceAll(v, "|", "")
		v = strings.TrimSpace(v)
		cleaned := re.ReplaceAllString(v, ",")
		profile, available, total, err := generateProfile(cleaned)
		if err != nil {
			return nil, err
		}

		availableInt, err := strconv.Atoi(available)
		if err != nil {
			return nil, fmt.Errorf("error converting available count %s to int %w", available, err)
		}

		totalInt, err := strconv.Atoi(total)
		if err != nil {
			return nil, fmt.Errorf("error converting total count %s to int %w", total, err)
		}
		ps := v1beta1.MigProfileStatus{
			MigProfiles: profile,
			Available:   availableInt,
			Total:       totalInt,
		}
		profileStatuses = append(profileStatuses, ps)

	}

	// need to verify if any MIG instances exist and include the info in status information under []IDs
	profileInstanceMap, err := generateMIGInstanceInfo(instanceInfo)
	if err != nil {
		return nil, fmt.Errorf("error profile instance map: %w", err)
	}
	for i, v := range profileStatuses {
		ids, ok := profileInstanceMap[v.ID]
		if ok {
			profileStatuses[i].VGPUID = ids
		}
	}
	return profileStatuses, nil
}

func generateProfileInstanceMap(info []string) map[string][]string {
	profileMap := make(map[string][]string)
	re := regexp.MustCompile(`\s+`)
	for _, v := range info {
		v = strings.ReplaceAll(v, "|", "")
		v = strings.TrimSpace(v)
		cleaned := re.ReplaceAllString(v, ",")
		elements := strings.Split(cleaned, ",")
		// profileID is element 4 in the array
		// instanceID is element 5 in the array

		ids := profileMap[elements[3]]
		if !slices.Contains(ids, elements[4]) {
			ids = append(ids, elements[4])
		}
		profileMap[elements[3]] = ids
	}

	return profileMap
}

// check if MIG status is NA
func isMigSupported(info string) bool {
	return !strings.Contains(info, NA)
}

// generateComputeInstanceList gets info about existing compute instances
// and this can be used during the deletion operation to identify a compute instance
// exists before triggering deletion of the same
func generateComputeInstanceList(info string) ([]string, error) {
	parsedInfo, err := parseMigInfo(info)
	if err != nil {
		return nil, fmt.Errorf("error parsing mig info: %w", err)
	}
	re := regexp.MustCompile(`\s+`)
	instanceIDList := make([]string, 0, len(parsedInfo))
	// extract 2nd element from line below containing compute instance information
	//|   0     13       MIG 1g.5gb           0         0          0:1     |
	for _, v := range parsedInfo {
		v = strings.ReplaceAll(v, "|", "")
		v = strings.TrimSpace(v)
		cleaned := re.ReplaceAllString(v, ",")
		elements := strings.Split(cleaned, ",")
		instanceIDList = append(instanceIDList, elements[1])
	}
	return instanceIDList, nil

}

// DisableMIGProfiles will disable all MIG instance associated with a MIG configuration
func DisableMIGProfiles(ex executor.Executor, mig *v1beta1.MigConfiguration) error {
	for _, profiles := range mig.Status.ProfileStatus {
		for _, id := range profiles.VGPUID {
			if err := DeleteMIGInstance(ex, mig.Spec.GPUAddress, id); err != nil {
				return fmt.Errorf("error deleting instance %s on gpu %s: %w", id, mig.Spec.GPUAddress, err)
			}
		}
	}
	return nil
}

// DeleteMIGInstance will query the details of the MIGInstances and trigger deletion of associated compute and MIG instance if it still exists
// the check is needed as its possible the CRD may be out of sync with underlying OS state of objects
func DeleteMIGInstance(ex executor.Executor, deviceAddress string, migInstanceID string) error {
	listComputeInstanceCommand := generateListMIGComputeInstanceCommand(deviceAddress)
	listComputeInstanceOutput, err := ex.Run(listComputeInstanceCommand, nil)
	if err != nil {
		return fmt.Errorf("error executing %s: %w", listComputeInstanceCommand, err)
	}
	computeInstanceIDs, err := generateComputeInstanceList(string(listComputeInstanceOutput))
	if err != nil {
		return fmt.Errorf("error generating compute instance IDs: %w", err)
	}

	// compute instance exists associated with MIG instance
	// this needs to be deleted before removing the MIG instance
	if slices.Contains(computeInstanceIDs, migInstanceID) {
		computeInstanceDeletionCommand := generateDeleteComputeInstanceCommand(deviceAddress, migInstanceID)
		_, err = ex.Run(computeInstanceDeletionCommand, nil)
		if err != nil {
			return fmt.Errorf("error deleting compute instance %s on device %s: %w", migInstanceID, deviceAddress, err)
		}
	}

	// check MIG instance exists before deleting the same
	listMIGInstanceCommand := generateListMigInstanceInfoCommand(deviceAddress)
	listMigInstanceOutput, err := ex.Run(listMIGInstanceCommand, nil)
	if err != nil {
		return fmt.Errorf("error executing %s: %w", listMIGInstanceCommand, err)
	}
	profileInstanceMap, err := generateMIGInstanceInfo(string(listMigInstanceOutput))
	if err != nil {
		return fmt.Errorf("error generating profileInstanceMap: %w", err)
	}

	for _, v := range profileInstanceMap {
		if slices.Contains(v, migInstanceID) {
			instanceDeletionCommand := generateDeleteGPUInstanceCommand(deviceAddress, migInstanceID)
			_, err = ex.Run(instanceDeletionCommand, nil)
			return err
		}
	}

	return nil
}

// CreateMIGInstance will create an MIG Instance and associate a compute instance with the same
func CreateMIGInstance(ex executor.Executor, deviceAddress string, profileID int) error {
	createInstanceCommand := generateCreateInstanceCommand(deviceAddress, profileID)
	createInstanceCommandOutput, err := ex.Run(createInstanceCommand, nil)
	if err != nil {
		return fmt.Errorf("error executing %s: %w", createInstanceCommand, err)
	}
	logrus.Debugf("instance creation results for GPU %s, profile %d %s", deviceAddress, profileID, string(createInstanceCommandOutput))
	return nil
}

// EnableMIGMode will enable MIG mode on the underlying GPU
func EnableMIGMode(ex executor.Executor, deviceAddress string) error {
	enableMIGModeCommand := generateEnableMIGModeCommand(deviceAddress)
	enableMIGModeCommandOutput, err := ex.Run(enableMIGModeCommand, nil)
	if err != nil {
		return fmt.Errorf("error executing %s: %w", enableMIGModeCommand, err)
	}
	logrus.Debugf("MIG mode enable results for GPU %s: %s", deviceAddress, string(enableMIGModeCommandOutput))
	return nil
}

// generateMigStatusCheckCommand generates the command to generate MIG status
func generateMigStatusCheckCommand(address string) string {
	return fmt.Sprintf("nvidia-smi -i %s --query-gpu=mig.mode.current --format=csv,noheader", address)
}

// generateListMIGProfileCommand returns the details of available profiles associated with a GPU
func generateListMIGProfileCommand(address string) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -lgip", address)
}

// generateListMIGComputeInstanceCommand will list the compute instances available on an underlying GPU. This is needed to ensure we check for existence of compute instance before deleting the same
func generateListMIGComputeInstanceCommand(address string) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -lci || exit 0", address)
}

// generateMIGInstanceInfoCommand returns the details of the available MIG Instances associated with a GPU
func generateListMigInstanceInfoCommand(address string) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -lgi || exit 0", address)
}

// generateCreateInstanceCommand generates the MIG instance and Compute instance
// associated with supplied profile
// nvidia-smi mig -i 0000:04:00.0 -cgi 19 -C
func generateCreateInstanceCommand(address string, profile int) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -cgi %d -C", address, profile)
}

// generateDeleteComputeInstanceCommand generates the command to delete the compute instance
// nvidia-smi mig -i 0000:04:00.0 -dci -gi 13
func generateDeleteComputeInstanceCommand(address, instance string) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -dci -gi %s", address, instance)
}

// generateDeleteGPUInstanceCommand generates the command to delete a MIG instance
// nvidia-smi mig -i 0000:04:00.0 -dgi -gi 13
func generateDeleteGPUInstanceCommand(address, instance string) string {
	return fmt.Sprintf("nvidia-smi mig -i %s -dgi -gi %s", address, instance)
}

func generateEnableMIGModeCommand(address string) string {
	return fmt.Sprintf("nvidia-smi -i %s -mig 1", address)
}
