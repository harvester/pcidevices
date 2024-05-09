package usbdevice

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/pflag"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/deviceplugins"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

func Register(ctx context.Context, usbDeviceCtrl v1beta1gen.USBDeviceController, usbDeviceClaimCtrl v1beta1gen.USBDeviceClaimController) error {
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		fmt.Println(err)
	}

	usbDeviceClaimController := &ClaimHandler{
		usbDeviceCache: usbDeviceCtrl.Cache(),
		usbClaimClient: usbDeviceClaimCtrl,
		virtClient:     virtClient,
		lock:           &sync.Mutex{},
		devicePlugin:   map[string]*deviceplugins.USBDevicePlugin{},
	}

	usbDeviceClaimCtrl.OnChange(ctx, "usbClaimClient-device-claim", usbDeviceClaimController.OnUSBDeviceClaimChanged)
	usbDeviceClaimCtrl.OnRemove(ctx, "usbClaimClient-device-claim-remove", usbDeviceClaimController.OnRemove)

	return nil
}
