package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"

	ctlnetwork "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io"

	"github.com/harvester/pcidevices/pkg/controller/gpudevice"
	"github.com/harvester/pcidevices/pkg/controller/nodecleanup"
	"github.com/harvester/pcidevices/pkg/controller/nodes"
	"github.com/harvester/pcidevices/pkg/controller/pcideviceclaim"
	"github.com/harvester/pcidevices/pkg/controller/sriovdevice"
	"github.com/harvester/pcidevices/pkg/crd"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
	"github.com/harvester/pcidevices/pkg/webhook"
)

func Setup(ctx context.Context, cfg *rest.Config, _ *runtime.Scheme) error {
	err := crd.Create(ctx, cfg)
	if err != nil {
		return fmt.Errorf("error setting up crds: %v", err)
	}

	clientFactory, err := client.NewSharedClientFactory(cfg, nil)
	if err != nil {
		return err
	}

	cacheFactory := cache.NewSharedCachedFactory(clientFactory, nil)

	rateLimit := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 5*time.Minute)
	workqueue.DefaultControllerRateLimiter()
	// Register scheme with the shared factory controller
	factory := controller.NewSharedControllerFactory(cacheFactory, &controller.SharedControllerFactoryOptions{
		DefaultRateLimiter: rateLimit,
		DefaultWorkers:     2,
	})
	if err != nil {
		return err
	}

	pciFactory, err := ctl.NewFactoryFromConfigWithOptions(cfg, &generic.FactoryOptions{
		SharedControllerFactory: factory,
	})

	if err != nil {
		return fmt.Errorf("error building pcidevice controllers: %s", err.Error())
	}

	coreFactory, err := ctlcore.NewFactoryFromConfigWithOptions(cfg, &ctlcore.FactoryOptions{
		SharedControllerFactory: factory,
	})

	if err != nil {
		return fmt.Errorf("error building core controllers: %v", err)
	}

	networkFactory, err := ctlnetwork.NewFactoryFromConfigWithOptions(cfg, &ctlnetwork.FactoryOptions{
		SharedControllerFactory: factory,
	})

	if err != nil {
		return fmt.Errorf("error building network controllers: %v", err)
	}

	pdCtl := pciFactory.Devices().V1beta1().PCIDevice()
	pdcCtl := pciFactory.Devices().V1beta1().PCIDeviceClaim()
	sriovCtl := pciFactory.Devices().V1beta1().SRIOVNetworkDevice()
	nodeCtl := pciFactory.Devices().V1beta1().Node()
	coreNodeCtl := coreFactory.Core().V1().Node()
	vlanCtl := networkFactory.Network().V1beta1().VlanConfig()
	sriovNetworkDeviceCache := sriovCtl.Cache()
	sriovGPUCtl := pciFactory.Devices().V1beta1().SRIOVGPUDevice()
	vGPUCtl := pciFactory.Devices().V1beta1().VGPUDevice()
	podCtl := coreFactory.Core().V1().Pod()
	RegisterIndexers(sriovNetworkDeviceCache)

	if err := pcideviceclaim.Register(ctx, pdcCtl, pdCtl); err != nil {
		return fmt.Errorf("error registering pcidevicclaim controllers :%v", err)
	}

	if err := nodes.Register(ctx, sriovCtl, pdCtl, nodeCtl, coreNodeCtl, vlanCtl.Cache(),
		sriovNetworkDeviceCache, pdcCtl, vGPUCtl, sriovGPUCtl); err != nil {
		return fmt.Errorf("error registering node controller: %v", err)
	}

	if err := sriovdevice.Register(ctx, sriovCtl, coreNodeCtl.Cache(), vlanCtl.Cache()); err != nil {
		return fmt.Errorf("error registering sriovdevice controller: %v", err)
	}

	if err := nodecleanup.Register(ctx, pdcCtl, pdCtl, coreNodeCtl); err != nil {
		return fmt.Errorf("error registering nodecleanup controller: %v", err)
	}

	if err := gpudevice.Register(ctx, sriovGPUCtl, vGPUCtl, pdcCtl, podCtl, cfg); err != nil {
		return fmt.Errorf("error registering gpudevice controller :%v", err)
	}
	if err := start.All(ctx, 2, coreFactory, networkFactory, pciFactory); err != nil {
		return fmt.Errorf("error starting controllers :%v", err)
	}

	// setup/delete node objects //
	if err := nodes.SetupNodeObjects(nodeCtl); err != nil {
		return fmt.Errorf("error setting up node object: %v", err)
	}

	w := webhook.New(ctx, cfg)
	if err := w.ListenAndServe(); err != nil {
		return fmt.Errorf("error starting webhook: %v", err)
	}

	<-ctx.Done()
	return nil
}
