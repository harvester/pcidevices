package fakeclients

import (
	"context"

	"github.com/harvester/harvester-network-controller/pkg/apis/network.harvesterhci.io/v1beta1"
	clientv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned/typed/network.harvesterhci.io/v1beta1"
	ctlnetworkv1beta1 "github.com/harvester/harvester-network-controller/pkg/generated/controllers/network.harvesterhci.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type VlanConfigCache func() clientv1beta1.VlanConfigInterface

func (c VlanConfigCache) Get(name string) (*v1beta1.VlanConfig, error) {
	return c().Get(context.TODO(), name, metav1.GetOptions{})
}

func (c VlanConfigCache) List(selector labels.Selector) ([]*v1beta1.VlanConfig, error) {
	list, err := c().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*v1beta1.VlanConfig, 0, len(list.Items))
	for _, node := range list.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (c VlanConfigCache) AddIndexer(_ string, _ ctlnetworkv1beta1.VlanConfigIndexer) {
	panic("implement me")
}

func (c VlanConfigCache) GetByIndex(_, _ string) ([]*v1beta1.VlanConfig, error) {
	panic("implement me")
}
