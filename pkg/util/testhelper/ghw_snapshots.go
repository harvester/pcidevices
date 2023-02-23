package testhelper

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jaypipes/ghw"
)

const (
	defaultTestNICSnapshotPrefix = "nicsnaps"
	defaultSnapPath              = "../../../tests/snapshots/linux-amd64-e147d239df014921c6cbb49fbc3d6c41.tar.gz"
	defaultTotalVFFile           = "sriov_totalvfs"
	defaultConfiguredVFFile      = "sriov_numvfs"
	defaultDevicePath            = "/sys/bus/pci/devices"
)

type SetupFakeSriov struct {
	Address  string
	TotalVFS string
	NumVFS   string
}

var (
	defaultSriovNICAddresses = []SetupFakeSriov{
		{
			Address:  "0000:04:00.0",
			TotalVFS: "63\n",
			NumVFS:   "4",
		},
		{
			Address:  "0000:04:00.1",
			TotalVFS: "63\n",
			NumVFS:   "0",
		},
	}
)

// SetupGHWNetworkSnapshot will setup a fake ghw client to be used for running local unit tests
func SetupGHWNetworkSnapshot() (*ghw.NetworkInfo, error) {
	tmpDir, err := ioutil.TempDir("/tmp", defaultTestNICSnapshotPrefix)
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("error creating tmp dir: %v", err)
	}

	network, err := ghw.Network(ghw.WithSnapshot(
		ghw.SnapshotOptions{
			Path: defaultSnapPath,
			Root: &tmpDir,
		},
	))

	if err != nil {
		return nil, fmt.Errorf("error setting up snapshot: %v", err)
	}

	// setup sriov_numvfs and total_vfs to help mimic fake sriov capable devices
	for _, v := range defaultSriovNICAddresses {
		if err := os.WriteFile(filepath.Join(tmpDir, defaultDevicePath, v.Address, defaultTotalVFFile), []byte(v.TotalVFS), fs.FileMode(os.O_WRONLY)); err != nil {
			return nil, fmt.Errorf("error creating totalvfs file for %s: %v", v, err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, defaultDevicePath, v.Address, defaultConfiguredVFFile), []byte(v.NumVFS), fs.FileMode(os.O_WRONLY)); err != nil {
			return nil, fmt.Errorf("error creating numvfs file for %s: %v", v, err)
		}
	}

	return network, nil

}
