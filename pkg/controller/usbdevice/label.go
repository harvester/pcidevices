package usbdevice

import (
	"fmt"
	"os"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type commonLabel struct {
	nodeName string
}

var cl *commonLabel

func init() {
	cl = &commonLabel{
		nodeName: os.Getenv(v1beta1.NodeEnvVarName),
	}
}

func (cl *commonLabel) labels() map[string]string {
	return map[string]string{
		"nodename": cl.nodeName,
	}
}

func (cl *commonLabel) selector() string {
	return fmt.Sprintf("nodename=%s", cl.nodeName)
}
