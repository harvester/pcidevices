package config

import (
	ctlnetwork "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"k8s.io/client-go/rest"
	"kubevirt.io/client-go/kubecli"

	ctldevices "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
)

type FactoryManager struct {
	DeviceFactory  *ctldevices.Factory
	CoreFactory    *ctlcore.Factory
	NetworkFactory *ctlnetwork.Factory

	KubevirtClient kubecli.KubevirtClient
	Cfg            *rest.Config
}

func NewFactoryManager(deviceFactory *ctldevices.Factory, coreFactory *ctlcore.Factory, networkFactory *ctlnetwork.Factory, kubevirtClient kubecli.KubevirtClient, cfg *rest.Config) *FactoryManager {
	return &FactoryManager{
		DeviceFactory:  deviceFactory,
		CoreFactory:    coreFactory,
		NetworkFactory: networkFactory,
		KubevirtClient: kubevirtClient,
		Cfg:            cfg,
	}
}
