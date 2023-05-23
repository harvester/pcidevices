package nodecleanup

import (
	"context"
	"fmt"

	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

const (
	wranglerFinalizer = "wrangler.cattle.io/PCIDeviceClaimOnRemove"
)

type Handler struct {
	pdcClient  v1beta1.PCIDeviceClaimClient
	pdClient   v1beta1.PCIDeviceClient
	nodeClient corecontrollers.NodeController
}

func (h *Handler) OnRemove(_ string, node *v1.Node) (*v1.Node, error) {
	if node == nil || node.DeletionTimestamp == nil {
		return node, nil
	}
	logrus.Debugf("cleaning pcidevices for node %s", node.Name)
	// Delete all of that Node's PCIDeviceClaims
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("error getting pdcs: %s", err)
		return node, err
	}
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName != node.Name {
			continue
		}
		pdcCopy := pdc.DeepCopy()
		if containsFinalizer(pdcCopy.Finalizers, wranglerFinalizer) {
			pdcCopy.Finalizers = removeFinalizer(pdcCopy.Finalizers, wranglerFinalizer)
			_, err := h.pdcClient.Update(pdcCopy)
			if err != nil {
				return node, fmt.Errorf("error removing finalizer: %v", err)
			}
		}

		err = h.pdcClient.Delete(pdc.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting pdc: %s", err)
			return node, err
		}
	}
	// Delete all of that Node's PCIDevices
	selector := fmt.Sprintf("nodename=%s", node.Name)
	pds, err := h.pdClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logrus.Errorf("error getting pds: %s", err)
		return node, err
	}
	for _, pd := range pds.Items {
		err = h.pdClient.Delete(pd.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("error deleting pd: %s", err)
			return node, err
		}
	}

	return node, nil
}

func Register(
	ctx context.Context,
	pdcClient v1beta1.PCIDeviceClaimController,
	pdClient v1beta1.PCIDeviceController,
	nodeClient corecontrollers.NodeController) error {
	handler := &Handler{
		pdcClient:  pdcClient,
		pdClient:   pdClient,
		nodeClient: nodeClient,
	}
	nodeClient.OnRemove(ctx, "node-remove", handler.OnRemove)
	return nil
}

func containsFinalizer(finalizers []string, finalizer string) bool {
	for _, v := range finalizers {
		if v == finalizer {
			return true
		}
	}
	return false
}

func removeFinalizer(finalizers []string, finalizer string) []string {
	for i, v := range finalizers {
		if v == finalizer {
			return append(finalizers[:i], finalizers[i+1:]...)
		}
	}
	return finalizers
}
