package deviceplugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseUSBSymLinkToPCIAddress(t *testing.T) {
	testcases := []struct {
		symLink  string
		expected string
	}{
		{
			symLink:  "../../../devices/pci0000:00/0000:00:02.0/0000:01:00.0/0000:02:01.0/usb2/2-1",
			expected: "0000:02:01.0",
		},
		{
			symLink:  "../../../devices/pci0000:00/0000:00:02.2/0000:04:00.0/usb3/3-1",
			expected: "0000:04:00.0",
		},
	}

	for _, testcase := range testcases {
		address := parseUSBSymLinkToPCIAddress(testcase.symLink)
		assert.Equal(t, testcase.expected, address)
	}
}
