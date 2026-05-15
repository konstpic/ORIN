package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/k8s-ui/k8s-ui/internal/k8s"
)

// kubeClient abstracts the process-local ClusterManager and per-registered
// RemoteCluster clients.
type kubeClient interface {
	MappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error)
	Apply(ctx context.Context, obj *unstructured.Unstructured, opts k8s.ApplyOptions) (*unstructured.Unstructured, error)
	Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error
	EnsureInformer(ctx context.Context, gvr schema.GroupVersionResource) error
	ListByLabel(ctx context.Context, gvr schema.GroupVersionResource, namespace string, sel labels.Selector) ([]*unstructured.Unstructured, error)
	// GetByName fetches a single live object by namespace+name, regardless of labels.
	GetByName(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error)
}

type localClient struct{ cm *k8s.ClusterManager }

func (l localClient) MappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	return l.cm.MappingFor(gvk)
}

func (l localClient) Apply(ctx context.Context, obj *unstructured.Unstructured, opts k8s.ApplyOptions) (*unstructured.Unstructured, error) {
	return l.cm.Apply(ctx, obj, opts)
}

func (l localClient) Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error {
	return l.cm.Delete(ctx, gvk, namespace, name)
}

func (l localClient) EnsureInformer(ctx context.Context, gvr schema.GroupVersionResource) error {
	return l.cm.EnsureInformer(ctx, gvr)
}

func (l localClient) ListByLabel(ctx context.Context, gvr schema.GroupVersionResource, namespace string, sel labels.Selector) ([]*unstructured.Unstructured, error) {
	_ = ctx
	return l.cm.ListByLabel(gvr, namespace, sel)
}

func (l localClient) GetByName(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return l.cm.LiveGet(ctx, gvr, namespace, name)
}

type remoteClient struct{ r *k8s.RemoteCluster }

func (r remoteClient) MappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	return r.r.MappingFor(gvk)
}

func (r remoteClient) Apply(ctx context.Context, obj *unstructured.Unstructured, opts k8s.ApplyOptions) (*unstructured.Unstructured, error) {
	return r.r.Apply(ctx, obj, opts)
}

func (r remoteClient) Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error {
	return r.r.Delete(ctx, gvk, namespace, name)
}

func (r remoteClient) EnsureInformer(ctx context.Context, gvr schema.GroupVersionResource) error {
	_ = ctx
	_ = gvr
	return nil
}

func (r remoteClient) ListByLabel(ctx context.Context, gvr schema.GroupVersionResource, namespace string, sel labels.Selector) ([]*unstructured.Unstructured, error) {
	return r.r.ListByLabel(ctx, gvr, namespace, sel)
}

func (r remoteClient) GetByName(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return r.r.LiveGet(ctx, gvr, namespace, name)
}
