package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// FieldManager is the SSA field manager identifier we own.
const FieldManager = "k8s-ui"

// ApplyOptions controls server-side apply behaviour.
type ApplyOptions struct {
	DryRun bool
	Force  bool
}

// Apply performs server-side apply on a single object.
func (cm *ClusterManager) Apply(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) (*unstructured.Unstructured, error) {
	mapping, err := cm.MappingFor(obj.GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("rest mapping: %w", err)
	}
	iface := cm.resourceIfaceFor(mapping, obj.GetNamespace())

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
func (cm *ClusterManager) Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error {
	mapping, err := cm.MappingFor(gvk)
	if err != nil {
		return err
	}
	iface := cm.resourceIfaceFor(mapping, namespace)
	policy := metav1.DeletePropagationForeground
	err = iface.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (cm *ClusterManager) resourceIfaceFor(mapping *meta.RESTMapping, namespace string) dynamic.ResourceInterface {
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := namespace
		if ns == "" {
			ns = "default"
		}
		return cm.dyn.Resource(mapping.Resource).Namespace(ns)
	}
	return cm.dyn.Resource(mapping.Resource)
}
