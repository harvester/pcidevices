// The PCI module is here to ask the kernel which drivers
// are currently bound to a given PCI device

package lspci

import (
	"errors"
	"os/exec"
	"strings"
)

func GetLspciOuptut(address string) (string, error) {
	output, err := exec.Command("lspci", "-s", address, "-v").Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func ExtractCurrentPCIDriver(lspciOutput string) (string, error) {
	// Grep for the 'Kernel driver in use: ' line:
	lines := strings.Split(lspciOutput, "\n")
	for _, line := range lines {
		line = strings.TrimLeft(line, "\t ") // remove whitespace
		if strings.HasPrefix(line, "Kernel driver in use:") {
			driver := strings.Trim(strings.Split(line, ":")[1], " \n\t")
			return driver, nil
		}
	}
	return "", errors.New("driver not found")
}

func ExtractKernelModules(lspciOutput string) ([]string, error) {
	// Grep for the 'Kernel modules: ' line:
	lines := strings.Split(lspciOutput, "\n")
	for _, line := range lines {
		line = strings.TrimLeft(line, "\t ") // remove whitespace
		if strings.HasPrefix(line, "Kernel modules:") {
			moduleLine := strings.Trim(strings.Split(line, ":")[1], " \n\t")
			modules := strings.Split(moduleLine, ", ")
			return modules, nil
		}
	}
	return nil, errors.New("modules not found")
}
