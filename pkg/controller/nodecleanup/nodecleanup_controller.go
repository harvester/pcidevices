package nodecleanup

import (
	"context"
	"fmt"

	"github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
