// The PCI module is here to ask the kernel which drivers
// are currently bound to a given PCI device

package lspci

import (
	"errors"
	"os/exec"
	"strings"
)

func lspci(address string) ([]byte, error) {
	output, err := exec.Command("lspci", "-Dvmmnks", address).Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}

func GetCurrentPCIDriver(address string) (string, error) {
	lspciOutput, err := lspci(address)
	if err != nil {
		return "", err
	}
	// Grep for the 'Driver: ' line:
	lines := strings.Split(string(lspciOutput), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Driver:") {
			driver := strings.Trim(strings.Split(line, ":")[1], " \n\t")
			return driver, nil
		}
	}
	return "", errors.New("driver not found")
}
