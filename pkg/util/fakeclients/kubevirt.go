package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kubevirtapiv1 "kubevirt.io/api/core/v1"

	kubevirtv1 "github.com/harvester/pcidevices/pkg/generated/clientset/versioned/typed/kubevirt.io/v1"
)

type KubeVirtClient func(namespace string) kubevirtv1.KubeVirtInterface

func (k KubeVirtClient) Update(d *kubevirtapiv1.KubeVirt) (*kubevirtapiv1.KubeVirt, error) {
	return k(d.Namespace).Update(context.TODO(), d, metav1.UpdateOptions{})
}

func (k KubeVirtClient) Get(namespace, name string, options metav1.GetOptions) (*kubevirtapiv1.KubeVirt, error) {
	return k(namespace).Get(context.TODO(), name, options)
}

func (k KubeVirtClient) Create(d *kubevirtapiv1.KubeVirt) (*kubevirtapiv1.KubeVirt, error) {
	return k(d.Namespace).Create(context.TODO(), d, metav1.CreateOptions{})
}

func (k KubeVirtClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return k(namespace).Delete(context.TODO(), name, *options)
}

func (k KubeVirtClient) List(namespace string, opts metav1.ListOptions) (*kubevirtapiv1.KubeVirtList, error) {
	return k(namespace).List(context.TODO(), opts)
}

func (k KubeVirtClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (k KubeVirtClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *kubevirtapiv1.KubeVirt, err error) {
	panic("implement me")
}

func (k KubeVirtClient) UpdateStatus(d *kubevirtapiv1.KubeVirt) (*kubevirtapiv1.KubeVirt, error) {
	return k(d.Namespace).Update(context.TODO(), d, metav1.UpdateOptions{})
}
