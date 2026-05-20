package k8s

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/orin/orin/internal/manifest"
)

// ResourceNode is a tree node used to build the UI's resource tree.
type ResourceNode struct {
	Object   *unstructured.Unstructured
	Children []*ResourceNode
}

// BuildTree resolves the application's resource tree: top-level managed
// objects (identified by tracking label) plus any descendants reachable via
// ownerReferences.
//
// The application destination namespace is not used as a filter: plain YAML
// often sets metadata.namespace explicitly (e.g. gitops-demo) while the app
// destination may be default or another value. ListByLabel with an empty
// namespace includes all namespaces for namespaced resources (see ClusterManager).
func (cm *ClusterManager) BuildTree(ctx context.Context, appName, namespace string) ([]*ResourceNode, error) {
	_ = namespace
	sel := labels.SelectorFromSet(labels.Set{manifest.TrackingLabel: appName})
	// We probe a fixed set of "common" GVRs. Optional GVRs (like CRDs that may
	// not be installed) are probed only if they exist in the cluster, and their
	// informers are pre-started with a short timeout to avoid blocking.
	candidates := defaultDiscoveryGVRs()
	for _, gvr := range optionalDiscoveryGVRs() {
		if cm.gvrExists(gvr) {
			// Pre-warm the informer with a bounded timeout so ListByLabel
			// (which calls EnsureInformer internally) returns immediately.
			_ = cm.ensureInformerWithTimeout(ctx, gvr, 5*time.Second)
			candidates = append(candidates, gvr)
		}
	}
	var roots []*unstructured.Unstructured
	for _, gvr := range candidates {
		objs, err := cm.ListByLabel(gvr, "", sel)
		if err != nil {
			continue
		}
		roots = append(roots, objs...)
	}

	// Walk owner refs by listing child kinds (Pods, ReplicaSets) cluster-wide
	// and grouping by ownerRef UID.
	children := map[string][]*unstructured.Unstructured{}
	for _, gvr := range defaultChildGVRs() {
		if err := cm.ensureInformerWithTimeout(ctx, gvr, 10*time.Second); err != nil {
			continue
		}
		objs, err := cm.ListByLabel(gvr, "", labels.Everything())
		if err != nil {
			continue
		}
		for _, o := range objs {
			for _, ref := range o.GetOwnerReferences() {
				children[string(ref.UID)] = append(children[string(ref.UID)], o)
			}
		}
	}

	// Collect the UIDs of all tracked root objects so we can detect orphaned
	// stale ReplicaSets below.
	rootUIDs := map[string]struct{}{}
	for _, r := range roots {
		rootUIDs[string(r.GetUID())] = struct{}{}
	}

	// Filter out stale ReplicaSets from the children map. Old ReplicaSets
	// left behind after a rollout have spec.replicas == 0 but still carry
	// the tracking label and ownerReferences. If not filtered here, they
	// appear as empty child nodes under the Deployment in the resource tree.
	for uid, objs := range children {
		var kept []*unstructured.Unstructured
		for _, o := range objs {
			if isStaleReplicaSet(o, rootUIDs) {
				continue
			}
			kept = append(kept, o)
		}
		children[uid] = kept
	}

	// Filter out stale ReplicaSets from roots (they may also carry the
	// tracking label and be listed as roots).
	var filteredRoots []*unstructured.Unstructured
	for _, r := range roots {
		if isStaleReplicaSet(r, rootUIDs) {
			continue
		}
		filteredRoots = append(filteredRoots, r)
	}

	var out []*ResourceNode
	for _, root := range filteredRoots {
		out = append(out, buildSubtree(root, children, map[string]struct{}{}))
	}
	return out, nil
}

// isStaleReplicaSet returns true when r is a ReplicaSet that:
//   - is owned by another object (e.g. a Deployment) that is itself a tracked root, AND
//   - has spec.replicas == 0 (scaled-down RS left over from a previous rollout).
func isStaleReplicaSet(r *unstructured.Unstructured, trackedRootUIDs map[string]struct{}) bool {
	if r.GetKind() != "ReplicaSet" {
		return false
	}
	// Check if owned by a tracked root (Deployment).
	ownedByTracked := false
	for _, ref := range r.GetOwnerReferences() {
		if _, ok := trackedRootUIDs[string(ref.UID)]; ok {
			ownedByTracked = true
			break
		}
	}
	if !ownedByTracked {
		return false
	}
	// Check desired replicas.
	replicas, found, err := unstructured.NestedInt64(r.Object, "spec", "replicas")
	if err != nil || !found {
		return false
	}
	return replicas == 0
}

func buildSubtree(o *unstructured.Unstructured, byOwner map[string][]*unstructured.Unstructured, visited map[string]struct{}) *ResourceNode {
	if _, ok := visited[string(o.GetUID())]; ok {
		return &ResourceNode{Object: o}
	}
	visited[string(o.GetUID())] = struct{}{}
	node := &ResourceNode{Object: o}
	for _, c := range byOwner[string(o.GetUID())] {
		node.Children = append(node.Children, buildSubtree(c, byOwner, visited))
	}
	return node
}

// defaultDiscoveryGVRs is the curated list of well-known built-in kinds
// that are always present in any Kubernetes cluster.
func defaultDiscoveryGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	}
}

// optionalDiscoveryGVRs lists CRDs that may or may not be installed.
// They are only probed if gvrExists() confirms the resource is available.
func optionalDiscoveryGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		// App of Apps: Argo CD Application CRDs.
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"},
	}
}

// gvrExists checks whether a GVR is available in the cluster via cached discovery.
// Returns false on any error or if the resource is not found.
func (cm *ClusterManager) gvrExists(gvr schema.GroupVersionResource) bool {
	lists, err := cm.disco.ServerPreferredResources()
	if err != nil {
		// Partial results are still usable; fall through.
		if lists == nil {
			return false
		}
	}
	groupVersion := gvr.Group + "/" + gvr.Version
	if gvr.Group == "" {
		groupVersion = gvr.Version
	}
	for _, list := range lists {
		if list.GroupVersion != groupVersion {
			continue
		}
		for _, r := range list.APIResources {
			if r.Name == gvr.Resource {
				return true
			}
		}
	}
	return false
}

// ensureInformerWithTimeout wraps EnsureInformer with a deadline so that
// unavailable CRDs do not block the caller indefinitely.
func (cm *ClusterManager) ensureInformerWithTimeout(ctx context.Context, gvr schema.GroupVersionResource, timeout time.Duration) error {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cm.EnsureInformer(tctx, gvr) }()
	select {
	case err := <-done:
		return err
	case <-tctx.Done():
		return tctx.Err()
	}
}

func defaultChildGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "", Version: "v1", Resource: "pods"},
	}
}

// GVKFromUnstructured returns the schema GVK of an unstructured object.
func GVKFromUnstructured(u *unstructured.Unstructured) schema.GroupVersionKind {
	return u.GroupVersionKind()
}

// GVKKey is a stable string identifier for a GVK.
func GVKKey(g schema.GroupVersionKind) string {
	return strings.Join([]string{g.Group, g.Version, g.Kind}, "/")
}
