package usbid

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/pcidevices/pkg/util/gousb"
)

func TestDescribeWithVendorAndProduct(t *testing.T) {
	testcases := []struct {
		vendor   string
		product  string
		expected string
	}{
		{"0951", "1666", "DataTraveler 100 G3/G4/SE9 G2/50 Kyson (Kingston Technology)"},
		{"1002", "1222", "Unknown 1002:1222"},
		{"1000", "1111", "Unknown (Speed Tech Corp.)"},
	}

	for _, tc := range testcases {
		vendor, _ := strconv.ParseInt(tc.vendor, 16, 64)
		product, _ := strconv.ParseInt(tc.product, 16, 64)

		output := DescribeWithVendorAndProduct(gousb.ID(vendor), gousb.ID(product)) // nolint:gosec

		assert.Equal(t, tc.expected, output)
	}
}
