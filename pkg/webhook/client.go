package webhook

import (
	"context"

	"github.com/rancher/wrangler/pkg/clients"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/schemes"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/client-go/rest"

	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io"

	ctlpcidevices "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
)

type Clients struct {
	clients.Clients
	CoreFactory      *ctlcore.Factory
	HarvesterFactory *ctlharvesterv1.Factory
	KubevirtFactory  *ctlkubevirtv1.Factory
	PCIFactory       *ctlpcidevices.Factory
}

func NewClient(ctx context.Context, rest *rest.Config, threadiness int) (*Clients, error) {
	clients, err := clients.NewFromConfig(rest, nil)
	if err != nil {
		return nil, err
	}

	if err := schemes.Register(v1.AddToScheme); err != nil {
		return nil, err
	}

	harvesterFactory, err := ctlharvesterv1.NewFactoryFromConfigWithOptions(rest, clients.FactoryOptions)
	if err != nil {
		return nil, err
	}

	if err = harvesterFactory.Start(ctx, threadiness); err != nil {
		return nil, err
	}

	kubevirtFactory, err := ctlkubevirtv1.NewFactoryFromConfigWithOptions(rest, clients.FactoryOptions)
	if err != nil {
		return nil, err
	}

	if err = kubevirtFactory.Start(ctx, threadiness); err != nil {
		return nil, err
	}

	coreFactory, err := ctlcore.NewFactoryFromConfigWithOptions(rest, clients.FactoryOptions)
	if err != nil {
		return nil, err
	}

	pciFactory, err := ctlpcidevices.NewFactoryFromConfigWithOptions(rest, clients.FactoryOptions)
	if err != nil {
		return nil, err
	}

	if err = coreFactory.Start(ctx, threadiness); err != nil {
		return nil, err
	}

	return &Clients{
		Clients:          *clients,
		HarvesterFactory: harvesterFactory,
		KubevirtFactory:  kubevirtFactory,
		CoreFactory:      coreFactory,
		PCIFactory:       pciFactory,
	}, nil
}
