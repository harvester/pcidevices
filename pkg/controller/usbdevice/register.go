package usbdevice

import (
	"context"

	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"kubevirt.io/client-go/kubecli"

	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	KubeVirtNamespace      = "harvester-system"
	KubeVirtResource       = "kubevirt"
	KubeVirtResourcePrefix = "kubevirt.io/"
)

func Register(ctx context.Context, usbDeviceCtrl v1beta1gen.USBDeviceController, usbDeviceClaimCtrl v1beta1gen.USBDeviceClaimController) error {
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		logrus.Errorf("failed to get kubevirt client: %v", err)
		return err
	}

	setupCommonLabels()

	handler := NewHandler(usbDeviceCtrl, usbDeviceClaimCtrl, virtClient)
	usbDeviceClaimController := NewClaimHandler(usbDeviceCtrl.Cache(), usbDeviceClaimCtrl, virtClient)

	usbDeviceClaimCtrl.OnChange(ctx, "usbClaimClient-device-claim", usbDeviceClaimController.OnUSBDeviceClaimChanged)
	usbDeviceClaimCtrl.OnRemove(ctx, "usbClaimClient-device-claim-remove", usbDeviceClaimController.OnRemove)
	relatedresource.WatchClusterScoped(ctx, "USBDeviceToClaimReconcile", handler.OnDeviceChange, usbDeviceClaimCtrl, usbDeviceCtrl)

	return nil
}
