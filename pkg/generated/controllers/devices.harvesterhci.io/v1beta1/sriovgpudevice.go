/*
Copyright 2022 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v1beta1

import (
	"context"
	"time"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type SRIOVGPUDeviceHandler func(string, *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error)

type SRIOVGPUDeviceController interface {
	generic.ControllerMeta
	SRIOVGPUDeviceClient

	OnChange(ctx context.Context, name string, sync SRIOVGPUDeviceHandler)
	OnRemove(ctx context.Context, name string, sync SRIOVGPUDeviceHandler)
	Enqueue(name string)
	EnqueueAfter(name string, duration time.Duration)

	Cache() SRIOVGPUDeviceCache
}

type SRIOVGPUDeviceClient interface {
	Create(*v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error)
	Update(*v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error)
	UpdateStatus(*v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*v1beta1.SRIOVGPUDevice, error)
	List(opts metav1.ListOptions) (*v1beta1.SRIOVGPUDeviceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.SRIOVGPUDevice, err error)
}

type SRIOVGPUDeviceCache interface {
	Get(name string) (*v1beta1.SRIOVGPUDevice, error)
	List(selector labels.Selector) ([]*v1beta1.SRIOVGPUDevice, error)

	AddIndexer(indexName string, indexer SRIOVGPUDeviceIndexer)
	GetByIndex(indexName, key string) ([]*v1beta1.SRIOVGPUDevice, error)
}

type SRIOVGPUDeviceIndexer func(obj *v1beta1.SRIOVGPUDevice) ([]string, error)

type sRIOVGPUDeviceController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewSRIOVGPUDeviceController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) SRIOVGPUDeviceController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &sRIOVGPUDeviceController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromSRIOVGPUDeviceHandlerToHandler(sync SRIOVGPUDeviceHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v1beta1.SRIOVGPUDevice
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v1beta1.SRIOVGPUDevice))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *sRIOVGPUDeviceController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v1beta1.SRIOVGPUDevice))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateSRIOVGPUDeviceDeepCopyOnChange(client SRIOVGPUDeviceClient, obj *v1beta1.SRIOVGPUDevice, handler func(obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error)) (*v1beta1.SRIOVGPUDevice, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *sRIOVGPUDeviceController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *sRIOVGPUDeviceController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *sRIOVGPUDeviceController) OnChange(ctx context.Context, name string, sync SRIOVGPUDeviceHandler) {
	c.AddGenericHandler(ctx, name, FromSRIOVGPUDeviceHandlerToHandler(sync))
}

func (c *sRIOVGPUDeviceController) OnRemove(ctx context.Context, name string, sync SRIOVGPUDeviceHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromSRIOVGPUDeviceHandlerToHandler(sync)))
}

func (c *sRIOVGPUDeviceController) Enqueue(name string) {
	c.controller.Enqueue("", name)
}

func (c *sRIOVGPUDeviceController) EnqueueAfter(name string, duration time.Duration) {
	c.controller.EnqueueAfter("", name, duration)
}

func (c *sRIOVGPUDeviceController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *sRIOVGPUDeviceController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *sRIOVGPUDeviceController) Cache() SRIOVGPUDeviceCache {
	return &sRIOVGPUDeviceCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *sRIOVGPUDeviceController) Create(obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	result := &v1beta1.SRIOVGPUDevice{}
	return result, c.client.Create(context.TODO(), "", obj, result, metav1.CreateOptions{})
}

func (c *sRIOVGPUDeviceController) Update(obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	result := &v1beta1.SRIOVGPUDevice{}
	return result, c.client.Update(context.TODO(), "", obj, result, metav1.UpdateOptions{})
}

func (c *sRIOVGPUDeviceController) UpdateStatus(obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	result := &v1beta1.SRIOVGPUDevice{}
	return result, c.client.UpdateStatus(context.TODO(), "", obj, result, metav1.UpdateOptions{})
}

func (c *sRIOVGPUDeviceController) Delete(name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), "", name, *options)
}

func (c *sRIOVGPUDeviceController) Get(name string, options metav1.GetOptions) (*v1beta1.SRIOVGPUDevice, error) {
	result := &v1beta1.SRIOVGPUDevice{}
	return result, c.client.Get(context.TODO(), "", name, result, options)
}

func (c *sRIOVGPUDeviceController) List(opts metav1.ListOptions) (*v1beta1.SRIOVGPUDeviceList, error) {
	result := &v1beta1.SRIOVGPUDeviceList{}
	return result, c.client.List(context.TODO(), "", result, opts)
}

func (c *sRIOVGPUDeviceController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), "", opts)
}

func (c *sRIOVGPUDeviceController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*v1beta1.SRIOVGPUDevice, error) {
	result := &v1beta1.SRIOVGPUDevice{}
	return result, c.client.Patch(context.TODO(), "", name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type sRIOVGPUDeviceCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *sRIOVGPUDeviceCache) Get(name string) (*v1beta1.SRIOVGPUDevice, error) {
	obj, exists, err := c.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v1beta1.SRIOVGPUDevice), nil
}

func (c *sRIOVGPUDeviceCache) List(selector labels.Selector) (ret []*v1beta1.SRIOVGPUDevice, err error) {

	err = cache.ListAll(c.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.SRIOVGPUDevice))
	})

	return ret, err
}

func (c *sRIOVGPUDeviceCache) AddIndexer(indexName string, indexer SRIOVGPUDeviceIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v1beta1.SRIOVGPUDevice))
		},
	}))
}

func (c *sRIOVGPUDeviceCache) GetByIndex(indexName, key string) (result []*v1beta1.SRIOVGPUDevice, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v1beta1.SRIOVGPUDevice, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v1beta1.SRIOVGPUDevice))
	}
	return result, nil
}

type SRIOVGPUDeviceStatusHandler func(obj *v1beta1.SRIOVGPUDevice, status v1beta1.SRIOVGPUDeviceStatus) (v1beta1.SRIOVGPUDeviceStatus, error)

type SRIOVGPUDeviceGeneratingHandler func(obj *v1beta1.SRIOVGPUDevice, status v1beta1.SRIOVGPUDeviceStatus) ([]runtime.Object, v1beta1.SRIOVGPUDeviceStatus, error)

func RegisterSRIOVGPUDeviceStatusHandler(ctx context.Context, controller SRIOVGPUDeviceController, condition condition.Cond, name string, handler SRIOVGPUDeviceStatusHandler) {
	statusHandler := &sRIOVGPUDeviceStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromSRIOVGPUDeviceHandlerToHandler(statusHandler.sync))
}

func RegisterSRIOVGPUDeviceGeneratingHandler(ctx context.Context, controller SRIOVGPUDeviceController, apply apply.Apply,
	condition condition.Cond, name string, handler SRIOVGPUDeviceGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &sRIOVGPUDeviceGeneratingHandler{
		SRIOVGPUDeviceGeneratingHandler: handler,
		apply:                           apply,
		name:                            name,
		gvk:                             controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterSRIOVGPUDeviceStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type sRIOVGPUDeviceStatusHandler struct {
	client    SRIOVGPUDeviceClient
	condition condition.Cond
	handler   SRIOVGPUDeviceStatusHandler
}

func (a *sRIOVGPUDeviceStatusHandler) sync(key string, obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type sRIOVGPUDeviceGeneratingHandler struct {
	SRIOVGPUDeviceGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *sRIOVGPUDeviceGeneratingHandler) Remove(key string, obj *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1beta1.SRIOVGPUDevice{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *sRIOVGPUDeviceGeneratingHandler) Handle(obj *v1beta1.SRIOVGPUDevice, status v1beta1.SRIOVGPUDeviceStatus) (v1beta1.SRIOVGPUDeviceStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.SRIOVGPUDeviceGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
