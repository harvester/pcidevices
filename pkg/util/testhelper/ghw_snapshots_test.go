package testhelper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SetupGHWNetworkSnapshot(t *testing.T) {
	assert := require.New(t)

	netInfo, err := SetupGHWNetworkSnapshot()
	assert.NoError(err, "expected no error during setup of Network snapshot")
	t.Log(len(netInfo.NICs))
	for _, v := range netInfo.NICs {
		t.Log(v.Name)
	}
}
