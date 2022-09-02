package lspci

import (
	"testing"
)

const lspciOutputIntel82599 = `22:00.0 Ethernet controller [0200]: Intel Corporation 82599 10 Gigabit Network Connection [8086:1557] (rev 01)
        Subsystem: Beijing Sinead Technology Co., Ltd. Device [1dcf:0317]
        Flags: fast devsel, IRQ 48
        Memory at b0000000 (64-bit, prefetchable) [disabled] [size=512K]
        I/O ports at ece0 [disabled] [size=32]
        Memory at b0080000 (64-bit, prefetchable) [disabled] [size=16K]
        Expansion ROM at dfe00000 [disabled] [size=512K]
        Capabilities: [40] Power Management version 3
        Capabilities: [50] MSI: Enable- Count=1/1 Maskable+ 64bit+
        Capabilities: [70] MSI-X: Enable- Count=64 Masked-
        Capabilities: [a0] Express Endpoint, MSI 00
        Capabilities: [e0] Vital Product Data
        Capabilities: [100] Advanced Error Reporting
        Capabilities: [140] Device Serial Number 80-61-5f-ff-ff-10-83-14
        Capabilities: [150] Alternative Routing-ID Interpretation (ARI)
        Capabilities: [160] Single Root I/O Virtualization (SR-IOV)
        Kernel driver in use: vfio-pci
        Kernel modules: ixgbe`

const lspciOutput82801IB = `00:1f.2 IDE interface: Intel Corporation 82801IB (ICH9) 2 port SATA Controller [IDE mode] (rev 02)
        Subsystem: Dell PowerEdge R610 SATA IDE Controller
        Kernel driver in use: ata_piix
        Kernel modules: ata_generic, pata_acpi, ata_piix`

func TestExtractCurrentPCIDriver(t *testing.T) {
	actual, err := ExtractCurrentPCIDriver(lspciOutputIntel82599)
	expected := "vfio-pci"
	if err != nil {
		t.Fatalf("%s", err)
	}
	if actual != expected {
		t.Fatalf("expected vfio-pci, got %s", actual)
	}
}

func TestExtractKernelModules(t *testing.T) {
	actual, err := ExtractKernelModules(lspciOutput82801IB)
	expected := []string{"ata_generic", "pata_acpi", "ata_piix"}
	if err != nil {
		t.Fatalf("%s", err)
	}

	if len(actual) != 3 {
		t.Fatal("expected slice of length 3")
	}

	if actual[0] != expected[0] ||
		actual[1] != expected[1] ||
		actual[2] != expected[2] {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}
