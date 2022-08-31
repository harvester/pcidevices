// The PCI module is here to ask the kernel which drivers
// are currently bound to a given PCI device

package lspci

import (
	"errors"
	"os/exec"
	"strings"
)

func lspci(address string) ([]byte, error) {
	output, err := exec.Command("lspci", "-s", address, "-v").Output()
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
	// Grep for the 'Kernel driver in use: ' line:
	lines := strings.Split(string(lspciOutput), "\n")
	for _, line := range lines {
		line = strings.TrimLeft(line, "\t ") // remove whitespace
		if strings.HasPrefix(line, "Kernel driver in use:") {
			driver := strings.Trim(strings.Split(line, ":")[1], " \n\t")
			return driver, nil
		}
	}
	return "", errors.New("driver not found")
}
