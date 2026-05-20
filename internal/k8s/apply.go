package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// FieldManager is the SSA field manager identifier we own.
const FieldManager = "orin"

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

	// Preprocess the object before applying
	obj = preprocessForApply(obj)

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

// preprocessForApply prepares an object for server-side apply by handling
// special cases like Secret stringData and removing duplicate ports.
func preprocessForApply(obj *unstructured.Unstructured) *unstructured.Unstructured {
	out := obj.DeepCopy()

	// Handle Secret: convert stringData to data (base64) before applying
	if out.GetKind() == "Secret" {
		if stringData, ok, _ := unstructured.NestedStringMap(out.Object, "stringData"); ok && len(stringData) > 0 {
			// Get existing data map or create new one
			data, _, _ := unstructured.NestedStringMap(out.Object, "data")
			if data == nil {
				data = make(map[string]string)
			}

			// Convert stringData to base64 and merge into data
			for k, v := range stringData {
				data[k] = base64Encode(v)
			}

			// Remove stringData and set data
			unstructured.RemoveNestedField(out.Object, "stringData")
			_ = unstructured.SetNestedStringMap(out.Object, data, "data")
		}
	}

	// Handle Deployment/StatefulSet: deduplicate ports
	if out.GetKind() == "Deployment" || out.GetKind() == "StatefulSet" {
		deduplicatePorts(out, "spec", "template", "spec", "containers")
		deduplicatePorts(out, "spec", "template", "spec", "initContainers")
	}

	return out
}

// deduplicatePorts removes duplicate port entries from containers
func deduplicatePorts(obj *unstructured.Unstructured, path ...string) {
	containers, ok, _ := unstructured.NestedSlice(obj.Object, path...)
	if !ok {
		return
	}

	for i, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		ports, ok := container["ports"].([]interface{})
		if !ok {
			continue
		}

		// Deduplicate ports by containerPort+protocol
		seen := make(map[string]bool)
		uniquePorts := []interface{}{}

		for _, p := range ports {
			port, ok := p.(map[string]interface{})
			if !ok {
				uniquePorts = append(uniquePorts, p)
				continue
			}

			// Create key from containerPort and protocol
			containerPort := fmt.Sprintf("%v", port["containerPort"])
			protocol := "TCP" // default
			if proto, ok := port["protocol"].(string); ok {
				protocol = proto
			}
			key := fmt.Sprintf("%s:%s", containerPort, protocol)

			if !seen[key] {
				seen[key] = true
				uniquePorts = append(uniquePorts, p)
			}
		}

		if len(uniquePorts) != len(ports) {
			container["ports"] = uniquePorts
			containers[i] = container
		}
	}

	_ = unstructured.SetNestedSlice(obj.Object, containers, path...)
}

// base64Encode encodes a string to base64
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// RestartDeployment performs a rolling restart by patching the pod template
// with a "restartedAt" annotation. This is equivalent to `kubectl rollout restart`.
func (cm *ClusterManager) RestartDeployment(ctx context.Context, namespace, name string) error {
	mapping, err := cm.MappingFor(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	if err != nil {
		return fmt.Errorf("rest mapping: %w", err)
	}
	iface := cm.resourceIfaceFor(mapping, namespace)

	now := time.Now().UTC().Format(time.RFC3339)
	patch := []byte(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"` + now + `"}}}}}`)
	_, err = iface.Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("restart deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

// GetOwnerReferences returns the owner references of a resource.
func (cm *ClusterManager) GetOwnerReferences(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) ([]metav1.OwnerReference, error) {
	mapping, err := cm.MappingFor(gvk)
	if err != nil {
		return nil, fmt.Errorf("rest mapping: %w", err)
	}
	iface := cm.resourceIfaceFor(mapping, namespace)
	obj, err := iface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj.GetOwnerReferences(), nil
}
