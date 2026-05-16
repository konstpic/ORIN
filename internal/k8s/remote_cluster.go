package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// RemoteCluster targets a single apiserver with dynamic client + discovery
// (no shared informers). Used for clusters registered from kubeconfig.
type RemoteCluster struct {
	dyn    dynamic.Interface
	disco  discovery.CachedDiscoveryInterface
	mapper meta.RESTMapper
	cs     kubernetes.Interface
}

// NewRemoteClusterFromKubeconfigYAML parses a kubeconfig and builds clients.
func NewRemoteClusterFromKubeconfigYAML(yaml string) (*RemoteCluster, error) {
	raw, err := clientcmd.Load([]byte(yaml))
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	rc, err := clientcmd.NewDefaultClientConfig(*raw, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return NewRemoteCluster(rc)
}

// NewRemoteCluster wires dynamic client + deferred RESTMapper.
func NewRemoteCluster(rc *rest.Config) (*RemoteCluster, error) {
	dyn, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}
	disco := memory.NewMemCacheClient(cs.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disco)
	return &RemoteCluster{dyn: dyn, disco: disco, mapper: mapper, cs: cs}, nil
}

// Clientset returns the typed kubernetes clientset for this cluster.
func (r *RemoteCluster) Clientset() kubernetes.Interface { return r.cs }

// MappingFor resolves a GVK to a REST mapping.
func (r *RemoteCluster) MappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	mapping, err := r.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return mapping, nil
	}
	if meta.IsNoMatchError(err) {
		r.disco.Invalidate()
		mapping, err = r.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	return mapping, err
}

// Apply performs server-side apply on a single object.
func (r *RemoteCluster) Apply(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) (*unstructured.Unstructured, error) {
	mapping, err := r.MappingFor(obj.GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("rest mapping: %w", err)
	}
	iface := remoteResourceIface(r.dyn, mapping, obj.GetNamespace())

	data, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}
	po := metav1.PatchOptions{FieldManager: FieldManager}
	if opts.Force {
		v := true
		po.Force = &v
	}
	if opts.DryRun {
		po.DryRun = []string{metav1.DryRunAll}
	}
	out, err := iface.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, po)
	if err != nil {
		return nil, fmt.Errorf("apply %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return out, nil
}

// Delete removes a single object.
func (r *RemoteCluster) Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error {
	mapping, err := r.MappingFor(gvk)
	if err != nil {
		return err
	}
	iface := remoteResourceIface(r.dyn, mapping, namespace)
	policy := metav1.DeletePropagationForeground
	err = iface.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// ListByLabel lists objects in namespace matching a label selector.
func (r *RemoteCluster) ListByLabel(ctx context.Context, gvr schema.GroupVersionResource, namespace string, sel labels.Selector) ([]*unstructured.Unstructured, error) {
	lo := metav1.ListOptions{}
	if sel != nil {
		lo.LabelSelector = sel.String()
	}
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = r.dyn.Resource(gvr).List(ctx, lo)
	} else {
		list, err = r.dyn.Resource(gvr).Namespace(namespace).List(ctx, lo)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].DeepCopy())
	}
	return out, nil
}

// LiveGet fetches a single live object by namespace+name from the remote cluster.
func (r *RemoteCluster) LiveGet(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	var iface dynamic.ResourceInterface
	if namespace == "" {
		iface = r.dyn.Resource(gvr)
	} else {
		iface = r.dyn.Resource(gvr).Namespace(namespace)
	}
	return iface.Get(ctx, name, metav1.GetOptions{})
}

func remoteResourceIface(dyn dynamic.Interface, mapping *meta.RESTMapping, namespace string) dynamic.ResourceInterface {
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := namespace
		if ns == "" {
			ns = "default"
		}
		return dyn.Resource(mapping.Resource).Namespace(ns)
	}
	return dyn.Resource(mapping.Resource)
}
