package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/rest"

	"github.com/rancher/wrangler/v3/pkg/generic"

	pcidevicev1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/devices.harvesterhci.io/v1beta1"
)

type NodeDevicesClient func() v1beta1.NodeInterface

func (n NodeDevicesClient) Update(d *pcidevicev1beta1.Node) (*pcidevicev1beta1.Node, error) {
	return n().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (n NodeDevicesClient) Get(name string, options metav1.GetOptions) (*pcidevicev1beta1.Node, error) {
	return n().Get(context.TODO(), name, options)
}

func (n NodeDevicesClient) Create(d *pcidevicev1beta1.Node) (*pcidevicev1beta1.Node, error) {
	return n().Create(context.TODO(), d, metav1.CreateOptions{})
}

func (n NodeDevicesClient) Delete(name string, options *metav1.DeleteOptions) error {
	return n().Delete(context.TODO(), name, *options)
}

func (n NodeDevicesClient) List(opts metav1.ListOptions) (*pcidevicev1beta1.NodeList, error) {
	return n().List(context.TODO(), opts)
}

func (n NodeDevicesClient) Watch(metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (n NodeDevicesClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *pcidevicev1beta1.Node, err error) {
	panic("implement me")
}

func (n NodeDevicesClient) UpdateStatus(d *pcidevicev1beta1.Node) (*pcidevicev1beta1.Node, error) {
	return n().Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (n NodeDevicesClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*pcidevicev1beta1.Node, *pcidevicev1beta1.NodeList], error) {
	panic("implement me")
}

type NodeDevicesCache func() v1beta1.NodeInterface

func (n NodeDevicesCache) Get(name string) (*pcidevicev1beta1.Node, error) {
	return n().Get(context.TODO(), name, metav1.GetOptions{})
}

func (n NodeDevicesCache) List(labels.Selector) ([]*pcidevicev1beta1.Node, error) {
	panic("implement me")
}

func (n NodeDevicesCache) AddIndexer(_ string, _ generic.Indexer[*pcidevicev1beta1.Node]) {
	panic("implement me")
}

func (n NodeDevicesCache) GetByIndex(_, _ string) ([]*pcidevicev1beta1.Node, error) {
	panic("implement me")
}
