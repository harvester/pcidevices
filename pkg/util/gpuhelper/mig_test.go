package gpuhelper

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	// nvidia-smi mig -i 00000000:01:00.0 -lgip
	availableProfiles = `+-----------------------------------------------------------------------------+
| GPU instance profiles:                                                      |
| GPU   Name             ID    Instances   Memory     P2P    SM    DEC   ENC  |
|                              Free/Total   GiB              CE    JPEG  OFA  |
|=============================================================================|
|   0  MIG 1g.5gb        19     7/7        4.75       No     14     0     0   |
|                                                             1     0     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 1g.5gb+me     20     1/1        4.75       No     14     1     0   |
|                                                             1     1     1   |
+-----------------------------------------------------------------------------+
|   0  MIG 1g.10gb       15     4/4        9.75       No     14     1     0   |
|                                                             1     0     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 2g.10gb       14     3/3        9.75       No     28     1     0   |
|                                                             2     0     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 3g.20gb        9     2/2        19.62      No     42     2     0   |
|                                                             3     0     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 4g.20gb        5     1/1        19.62      No     56     2     0   |
|                                                             4     0     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 7g.40gb        0     1/1        39.50      No     98     5     0   |
|                                                             7     1     1   |
+-----------------------------------------------------------------------------+`

	// nvidia-smi --query-gpu=name,pci.bus_id,mig.mode.current,mig.mode.pending --format=csv,noheader
	migStatusSupported   = `NVIDIA A100-PCIE-40GB, 00000000:04:00.0, Enabled, Enabled`
	migStatusUnsupported = `NVIDIA A2, 00000000:08:00.0, [N/A], [N/A]`

	// nvidia-smi mig -i 00000000:01:00.0 -lgi
	listInstances = `+-------------------------------------------------------+
| GPU instances:                                        |
| GPU   Name             Profile  Instance   Placement  |
|                          ID       ID       Start:Size |
|=======================================================|
|   0  MIG 3g.47gb          9        1          0:4     |
+-------------------------------------------------------+
|   0  MIG 3g.47gb          9        2          4:4     |
+-------------------------------------------------------+`

	// sample nvidia-smi mig  -i 00000000:01:00.0 -lgip when profiles have been created
	profilesCreated = `+-----------------------------------------------------------------------------+
| GPU instance profiles:                                                      |
| GPU   Name             ID    Instances   Memory     P2P    SM    DEC   ENC  |
|                              Free/Total   GiB              CE    JPEG  OFA  |
|=============================================================================|
|   0  MIG 1g.12gb       19     0/7        10.62      No     16     1     0   |
|                                                             1     1     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 1g.12gb+me    20     0/1        10.62      No     16     1     0   |
|                                                             1     1     1   |
+-----------------------------------------------------------------------------+
|   0  MIG 1g.24gb       15     0/4        21.50      No     26     1     0   |
|                                                             1     1     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 2g.24gb       14     0/3        21.50      No     32     2     0   |
|                                                             2     2     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 3g.47gb        9     0/2        46.12      No     60     3     0   |
|                                                             3     3     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 4g.47gb        5     0/1        46.12      No     64     4     0   |
|                                                             4     4     0   |
+-----------------------------------------------------------------------------+
|   0  MIG 7g.94gb        0     0/1        92.62      No     132    7     0   |
|                                                             8     7     1   |
+-----------------------------------------------------------------------------+`

	listComputeInstance = `+--------------------------------------------------------------------+
| Compute instances:                                                 |
| GPU     GPU       Name             Profile   Instance   Placement  |
|       Instance                       ID        ID       Start:Size |
|         ID                                                         |
|====================================================================|
|   0     13       MIG 1g.5gb           0         0          0:1     |
+--------------------------------------------------------------------+`
)

func Test_parseMigInfo(t *testing.T) {
	assert := require.New(t)
	info, err := parseMigInfo(availableProfiles)
	assert.NoError(err, "expected no error while reading profile info")
	re := regexp.MustCompile(`\s+`)
	for _, v := range info {
		v = strings.ReplaceAll(v, "|", "")
		v = strings.TrimSpace(v)
		cleaned := re.ReplaceAllString(v, ",")
		t.Log(strings.Split(cleaned, ","))
	}

	profiles, err := generateProfileSpec(availableProfiles)
	assert.NoError(err)
	t.Log(profiles)
}

func Test_parseMigProfileStatus(t *testing.T) {
	assert := require.New(t)
	ps, err := GenerateProfileStatus(profilesCreated, listInstances)
	assert.NoError(err, "expected no error during profile status generation")
	// based on sample data all profiles are exhausted so available count for all profiles should be 0
	for _, v := range ps {
		assert.Equal(0, v.Available, "expected 0 instance capacity for instance", v.Name)
	}

	// expected to find 2 instances for profile 9
	for _, v := range ps {
		if v.ID == 9 {
			assert.Len(v.VGPUID, 2, "expected to find 2 vGPU ID's based on sample data")
		}
	}
}

func Test_isMigSupported(t *testing.T) {
	assert := require.New(t)
	assert.True(isMigSupported(migStatusSupported), "expected MIG to be supported")
	assert.False(isMigSupported(migStatusUnsupported), "expected MIG to be unsupported")
}

func Test_GenerateComputeInstanceList(t *testing.T) {
	assert := require.New(t)
	computeInstanceList, err := generateComputeInstanceList(listComputeInstance)
	assert.NoError(err)
	assert.Len(computeInstanceList, 1, "expected to find 1 element in compute instance list")
	t.Log(computeInstanceList)
}
