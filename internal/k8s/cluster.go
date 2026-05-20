// Package k8s holds the Kubernetes integration: cluster connectivity,
// shared informer caches, diff engine, and server-side apply.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memcached "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/orin/orin/internal/config"
)

// ClusterManager holds the per-cluster K8s clients and informer factory.
// The MVP only manages the cluster the controller runs in.
type ClusterManager struct {
	cfg    *config.Config
	rest   *rest.Config

	clientset      kubernetes.Interface
	dyn            dynamic.Interface
	disco          discovery.CachedDiscoveryInterface
	mapper         meta.RESTMapper

	factory        dynamicinformer.DynamicSharedInformerFactory
	informersMu    sync.Mutex
	informers      map[schema.GroupVersionResource]struct{}

	stopCh         chan struct{}
}

// NewClusterManager builds a ClusterManager from in-cluster or kubeconfig.
func NewClusterManager(cfg *config.Config) (*ClusterManager, error) {
	rc, err := loadRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}
	dc, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("dynamic: %w", err)
	}
	disco := memcached.NewMemCacheClient(cs.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disco)

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, 0, "", nil)

	stop := make(chan struct{})
	cm := &ClusterManager{
		cfg:       cfg,
		rest:      rc,
		clientset: cs,
		dyn:       dc,
		disco:     disco,
		mapper:    mapper,
		factory:   factory,
		informers: make(map[schema.GroupVersionResource]struct{}),
		stopCh:    stop,
	}
	go cm.refreshDiscoveryLoop()
	return cm, nil
}

// ServerURL returns the REST endpoint for the configured cluster.
func (cm *ClusterManager) ServerURL() string { return cm.rest.Host }

// Discovery returns the cached discovery client.
func (cm *ClusterManager) Discovery() discovery.CachedDiscoveryInterface { return cm.disco }

// Dynamic returns the dynamic client.
func (cm *ClusterManager) Dynamic() dynamic.Interface { return cm.dyn }

// Clientset returns the typed clientset.
func (cm *ClusterManager) Clientset() kubernetes.Interface { return cm.clientset }

// RESTMapper returns the discovery-backed REST mapper.
func (cm *ClusterManager) RESTMapper() meta.RESTMapper { return cm.mapper }

// MappingFor returns the REST mapping for a Kind, refreshing discovery on miss.
func (cm *ClusterManager) MappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	mapping, err := cm.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return mapping, nil
	}
	if meta.IsNoMatchError(err) {
		cm.disco.Invalidate()
		mapping, err = cm.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	return mapping, err
}

// EnsureInformer lazily starts an informer for the given GVR.
// It respects ctx for the WaitForCacheSync call so that callers can enforce
// timeouts and avoid blocking indefinitely when a CRD is not installed.
func (cm *ClusterManager) EnsureInformer(ctx context.Context, gvr schema.GroupVersionResource) error {
	cm.informersMu.Lock()
	defer cm.informersMu.Unlock()
	if _, ok := cm.informers[gvr]; ok {
		return nil
	}
	cm.factory.ForResource(gvr) // registers informer
	cm.factory.Start(cm.stopCh)

	// Use a combined stop channel: close either when the manager stops or when ctx is done.
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-cm.stopCh:
		case <-ctx.Done():
		}
		close(stopCh)
	}()

	synced := cm.factory.WaitForCacheSync(stopCh)
	if !synced[gvr] {
		// Some GVRs may legitimately fail (no permission, CRD not installed, etc.)
		slog.Warn("informer cache did not sync", "gvr", gvr)
	}
	cm.informers[gvr] = struct{}{}
	slog.Debug("informer started", "gvr", gvr)
	return nil
}

// ListByLabel returns cached objects matching the label selector for a GVR.
// It starts and syncs the informer for gvr if not already running; use a
// context with a deadline to avoid blocking when the GVR is unavailable.
func (cm *ClusterManager) ListByLabel(gvr schema.GroupVersionResource, namespace string, sel labels.Selector) ([]*unstructured.Unstructured, error) {
	if err := cm.EnsureInformer(context.Background(), gvr); err != nil {
		return nil, err
	}
	inf := cm.factory.ForResource(gvr)
	store := inf.Informer().GetStore()
	objs := store.List()
	var out []*unstructured.Unstructured
	for _, o := range objs {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if namespace != "" && u.GetNamespace() != namespace {
			continue
		}
		if sel != nil && !sel.Matches(labels.Set(u.GetLabels())) {
			continue
		}
		out = append(out, u.DeepCopy())
	}
	return out, nil
}

// LiveGet fetches a single live object from the dynamic client (informer
// miss path).
func (cm *ClusterManager) LiveGet(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	var iface dynamic.ResourceInterface
	if namespace == "" {
		iface = cm.dyn.Resource(gvr)
	} else {
		iface = cm.dyn.Resource(gvr).Namespace(namespace)
	}
	return iface.Get(ctx, name, metav1.GetOptions{})
}

// Close stops informers and frees resources.
func (cm *ClusterManager) Close() {
	close(cm.stopCh)
}

func (cm *ClusterManager) refreshDiscoveryLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-cm.stopCh:
			return
		case <-t.C:
			cm.disco.Invalidate()
		}
	}
}

func loadRESTConfig(cfg *config.Config) (*rest.Config, error) {
	if cfg.InCluster {
		return rest.InClusterConfig()
	}
	if cfg.KubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", cfg.KubeconfigPath)
	}
	// Try in-cluster, fall back to default kubeconfig location.
	rc, err := rest.InClusterConfig()
	if err == nil {
		return rc, nil
	}
	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, err
	}
	loadRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadRules, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// Ensure the deferred mapper implements meta.RESTMapper at compile time.
var _ meta.RESTMapper = (*restmapper.DeferredDiscoveryRESTMapper)(nil)
