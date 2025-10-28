package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	ctlcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"kubevirt.io/client-go/kubecli"

	ctlnetwork "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io"

	"github.com/harvester/pcidevices/pkg/config"
	"github.com/harvester/pcidevices/pkg/controller/gpudevice"
	"github.com/harvester/pcidevices/pkg/controller/nodecleanup"
	"github.com/harvester/pcidevices/pkg/controller/nodes"
	"github.com/harvester/pcidevices/pkg/controller/pcideviceclaim"
	"github.com/harvester/pcidevices/pkg/controller/sriovdevice"
	"github.com/harvester/pcidevices/pkg/controller/usbdevice"
	"github.com/harvester/pcidevices/pkg/controller/virtualmachine"
	"github.com/harvester/pcidevices/pkg/crd"
	ctldevices "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io"
	ctlkubevirt "github.com/harvester/pcidevices/pkg/generated/controllers/kubevirt.io"
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

	k8sclient, err := kubernetes.NewForConfig(cfg)
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

	deviceFactory, err := ctldevices.NewFactoryFromConfigWithOptions(cfg, &generic.FactoryOptions{
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

	kubevirtFactory, err := ctlkubevirt.NewFactoryFromConfigWithOptions(cfg, &ctlkubevirt.FactoryOptions{
		SharedControllerFactory: factory,
	})
	if err != nil {
		return fmt.Errorf("error building kubevirt controllers: %v", err)
	}

	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("cannot obtain KubeVirt client: %v", err)
	}

	management := config.NewFactoryManager(
		deviceFactory,
		coreFactory,
		networkFactory,
		kubevirtFactory,
		virtClient,
		cfg,
	)

	nodeCtl := deviceFactory.Devices().V1beta1().Node()

	RegisterIndexers(management)

	registers := []func(context.Context, *config.FactoryManager) error{
		pcideviceclaim.Register,
		usbdevice.Register,
		nodes.Register,
		sriovdevice.Register,
		gpudevice.Register,
		virtualmachine.Register,
	}

	for _, register := range registers {
		if err := register(ctx, management); err != nil {
			return fmt.Errorf("error registering controller: %v", err)
		}
	}

	// need to ensure leader election runs for nodecleanup controller
	go leader.RunOrDie(ctx, "harvester-system", "pcidevices-node-cleanup", k8sclient, func(ctx context.Context) {
		logrus.Info("starting leader election for nodecleanup controller")
		if err := nodecleanup.Register(ctx, management); err != nil {
			panic(err)
		}
		<-ctx.Done()
	})

	if err := start.All(ctx, 2, coreFactory, networkFactory, deviceFactory, kubevirtFactory); err != nil {
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
