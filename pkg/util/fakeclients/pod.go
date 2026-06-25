package fakeclients

import (
	"context"

	corev1type "github.com/harvester/harvester/pkg/generated/clientset/versioned/typed/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

type PodCache func(string) corev1type.PodInterface

func (p PodCache) Get(namespace, name string) (*corev1.Pod, error) {
	return p(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (p PodCache) List(namespace string, selector labels.Selector) ([]*corev1.Pod, error) {
	list, err := p(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*corev1.Pod, 0, len(list.Items))
	for _, node := range list.Items {
		obj := node
		result = append(result, &obj)
	}
	return result, err
}

func (p PodCache) AddIndexer(_ string, _ generic.Indexer[*corev1.Pod]) {
	panic("implement me")
}

func (p PodCache) GetByIndex(indexName, key string) ([]*corev1.Pod, error) {
	switch indexName {
	case pcidevicev1beta1.GPUPodsByNodeName:
		podList, err := p.List(metav1.NamespaceAll, labels.NewSelector())
		if err != nil {
			return nil, err
		}

		var filteredList []*corev1.Pod
		for _, v := range podList {
			if v.Spec.NodeName == key && v.Status.Phase == corev1.PodRunning {
				for _, container := range v.Spec.Containers {
					_, ok := container.Resources.Requests[corev1.ResourceName(pcidevicev1beta1.GPUResourceName)]
					if ok {
						filteredList = append(filteredList, v)
						break
					}
				}
			}
		}
		return filteredList, nil
	default:
		panic("implement me")
	}
}
