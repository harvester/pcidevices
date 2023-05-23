package nodes

import (
	"context"
	"fmt"
	"os"
	"time"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/controller/pcidevice"
	"github.com/harvester/pcidevices/pkg/controller/sriovdevice"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/nichelper"
)

const (
	defaultRequeuePeriod = 30 * time.Second
)

type handler struct {
	ctx                     context.Context
	sriovCache              ctl.SRIOVNetworkDeviceCache
	sriovClient             ctl.SRIOVNetworkDeviceClient
	pciDeviceClient         ctl.PCIDeviceClient
	pciDeviceCache          ctl.PCIDeviceCache
	nodeName                string
	vlanConfigCache         ctlnetworkv1beta1.VlanConfigCache
	coreNodeCache           ctlcorev1.NodeCache
	nodeCtl                 ctl.NodeController
	sriovNetworkDeviceCache ctl.SRIOVNetworkDeviceCache
}

const (
	reconcilePCIDevices = "reconcile-pcidevices"
)

func Register(ctx context.Context, sriovCtl ctl.SRIOVNetworkDeviceController, pciDeviceCtl ctl.PCIDeviceController, nodeCtl ctl.NodeController, coreNodeCache ctlcorev1.NodeCache, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache, sriovNetworkDeviceCache ctl.SRIOVNetworkDeviceCache) error {
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)
	h := &handler{
		ctx:                     ctx,
		sriovCache:              sriovCtl.Cache(),
		sriovClient:             sriovCtl,
		pciDeviceClient:         pciDeviceCtl,
		pciDeviceCache:          pciDeviceCtl.Cache(),
		nodeName:                nodeName,
		coreNodeCache:           coreNodeCache,
		vlanConfigCache:         vlanConfigCache,
		nodeCtl:                 nodeCtl,
		sriovNetworkDeviceCache: sriovNetworkDeviceCache,
	}

	nodeCtl.OnChange(ctx, reconcilePCIDevices, h.reconcileNodeDevices)
	return nil
}

func (h *handler) reconcileNodeDevices(name string, node *v1beta1.Node) (*v1beta1.Node, error) {
	if node == nil || node.DeletionTimestamp != nil || node.Name != h.nodeName {
		return node, nil
	}

	pci, err := ghw.PCI()
	if err != nil {
		return node, fmt.Errorf("error listing pcidevices: %v", err)
	}

	skipAddresses, err := nichelper.IdentifyHarvesterManagedNIC(h.nodeName, h.coreNodeCache, h.vlanConfigCache)
	if err != nil {
		return node, fmt.Errorf("error identifying management nics: %v", err)
	}

	pciBridgeAddresses := pcidevice.IdentifyPCIBridgeDevices(pci)
	skipAddresses = append(skipAddresses, pciBridgeAddresses...)

	pciHandler := pcidevice.NewHandler(h.pciDeviceClient, pci, h.coreNodeCache, h.vlanConfigCache, h.sriovNetworkDeviceCache, skipAddresses)
	err = pciHandler.ReconcilePCIDevices(h.nodeName)
	if err != nil {
		return nil, fmt.Errorf("error reconciling pcidevices for node %s: %v", h.nodeName, err)
	}

	// additional steps for sriov reconcile
	sriovHelper := sriovdevice.NewHandler(h.ctx, h.sriovCache, h.sriovClient, h.nodeName, h.coreNodeCache, h.vlanConfigCache)
	err = sriovHelper.SetupSriovDevices()
	if err != nil {
		return nil, fmt.Errorf("error setting up sriov devices for node %s: %v", h.nodeName, err)
	}
	//
	h.nodeCtl.EnqueueAfter(name, defaultRequeuePeriod)
	return node, err
}

func SetupNodeObjects(nodeCtl ctl.NodeController) error {
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)
	_, err := nodeCtl.Cache().Get(nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			nodeObj := &v1beta1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1beta1.NodeSpec{},
			}
			_, err := nodeCtl.Create(nodeObj)
			return err
		}
		return err
	}

	return nil
}
