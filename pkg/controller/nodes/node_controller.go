package nodes

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/harvester/pcidevices/pkg/controller/gpudevice"

	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	"github.com/jaypipes/ghw"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

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
	ctx                      context.Context
	sriovCache               ctl.SRIOVNetworkDeviceCache
	sriovClient              ctl.SRIOVNetworkDeviceClient
	pciDeviceClient          ctl.PCIDeviceClient
	pciDeviceCache           ctl.PCIDeviceCache
	nodeName                 string
	vlanConfigCache          ctlnetworkv1beta1.VlanConfigCache
	coreNodeCache            ctlcorev1.NodeCache
	coreNodeCtl              ctlcorev1.NodeController
	nodeCtl                  ctl.NodeController
	sriovNetworkDeviceCache  ctl.SRIOVNetworkDeviceCache
	vGPUController           ctl.VGPUDeviceController
	pciDeviceClaimController ctl.PCIDeviceClaimController
	sriovGPUController       ctl.SRIOVGPUDeviceController
}

const (
	reconcilePCIDevices = "reconcile-pcidevices"
)

func Register(ctx context.Context, sriovCtl ctl.SRIOVNetworkDeviceController, pciDeviceCtl ctl.PCIDeviceController,
	nodeCtl ctl.NodeController, coreNodeCtl ctlcorev1.NodeController, vlanConfigCache ctlnetworkv1beta1.VlanConfigCache,
	sriovNetworkDeviceCache ctl.SRIOVNetworkDeviceCache, pciDeviceClaimController ctl.PCIDeviceClaimController, vGPUController ctl.VGPUDeviceController,
	sriovGPUController ctl.SRIOVGPUDeviceController) error {
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)
	h := &handler{
		ctx:                      ctx,
		sriovCache:               sriovCtl.Cache(),
		sriovClient:              sriovCtl,
		pciDeviceClient:          pciDeviceCtl,
		pciDeviceCache:           pciDeviceCtl.Cache(),
		nodeName:                 nodeName,
		coreNodeCache:            coreNodeCtl.Cache(),
		coreNodeCtl:              coreNodeCtl,
		vlanConfigCache:          vlanConfigCache,
		nodeCtl:                  nodeCtl,
		sriovNetworkDeviceCache:  sriovNetworkDeviceCache,
		vGPUController:           vGPUController,
		pciDeviceClaimController: pciDeviceClaimController,
		sriovGPUController:       sriovGPUController,
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
	gpuhelper, _ := gpudevice.NewHandler(h.ctx, h.sriovGPUController, h.vGPUController, h.pciDeviceClaimController, nil, nil, nil)
	err = gpuhelper.SetupSRIOVGPUDevices()
	if err != nil {
		return nil, fmt.Errorf("error setting up SRIOV GPU devices for node %s: %v", h.nodeName, err)
	}

	err = gpuhelper.SetupVGPUDevices()
	if err != nil {
		return nil, fmt.Errorf("error setting VGPU devices for node %s: %v", h.nodeName, err)
	}

	err = checkAndUpdateNodeLabels(h.nodeName, h.coreNodeCtl.Cache(), h.coreNodeCtl, h.sriovGPUController.Cache())
	if err != nil {
		return nil, fmt.Errorf("error updating node labels for node %s: %v", h.nodeName, err)
	}

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

// checkAndUpdateNodeLabels checks if a node has SRIOV capable GPU's and updates label
// sriovgpu.harvesterhci.io/driver-needed=true
// this label is in turn used by the NVIDIA driver daemonset to ensure scheduling to this node
func checkAndUpdateNodeLabels(nodeName string, nodeCache ctlcorev1.NodeCache, nodeClient ctlcorev1.NodeClient, sriovGPUCache ctl.SRIOVGPUDeviceCache) error {
	set := map[string]string{
		v1beta1.NodeKeyName: nodeName,
	}

	existingGPUs, err := sriovGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return err
	}

	var removeLabel bool
	if len(existingGPUs) == 0 {
		removeLabel = true
	} else {
		removeLabel = false
	}

	nodeObj, err := nodeCache.Get(nodeName)
	if err != nil {
		return err
	}

	if nodeObj.Labels == nil {
		nodeObj.Labels = make(map[string]string)
	}

	nodeObjCopy := nodeObj.DeepCopy()
	_, labelPresent := nodeObj.Labels[v1beta1.NvidiaDriverNeededKey]
	if labelPresent && removeLabel {
		delete(nodeObj.Labels, v1beta1.NvidiaDriverNeededKey)
	}
	if !labelPresent && !removeLabel {
		nodeObj.Labels[v1beta1.NvidiaDriverNeededKey] = "true"
	}

	// update object if needed
	if !reflect.DeepEqual(nodeObj.Labels, nodeObjCopy.Labels) {
		_, err = nodeClient.Update(nodeObj)
	}

	return err
}
