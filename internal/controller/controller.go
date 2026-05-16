// Package controller is the reconciliation engine. It owns two workqueues:
// one for status reconcile, one for sync execution. The status reconcile
// loop computes desired vs live and updates application_status; the sync
// loop drains pending SyncOperation rows.
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/k8s-ui/k8s-ui/internal/config"
	"github.com/k8s-ui/k8s-ui/internal/crypto"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/k8s"
	"github.com/k8s-ui/k8s-ui/internal/manifest"
	"github.com/k8s-ui/k8s-ui/internal/reposerver"
	"github.com/k8s-ui/k8s-ui/internal/store"
	"github.com/k8s-ui/k8s-ui/internal/ws"
)

// Controller orchestrates reconcile + sync execution for all applications.
type Controller struct {
	cfg   *config.Config
	store *store.Store
	k8s   *k8s.ClusterManager
	repo  *reposerver.Server
	hub   *ws.Hub
	cipher *crypto.Cipher

	remoteMu      sync.Mutex
	remoteClients map[string]*k8s.RemoteCluster

	statusQ *workqueue
	syncQ   *workqueue

	mu      sync.RWMutex
	tracked map[string]struct{} // app names actively in queues

	// manualApplyGrace tracks the timestamp of the last manual live-apply per app.
	// Auto-sync is suppressed for these apps during the grace period to let
	// user edits survive without being immediately reverted by self-heal.
	manualApplyMu    sync.Mutex
	manualApplyGrace map[string]time.Time
}

// New constructs the Controller.
func New(cfg *config.Config, st *store.Store, cm *k8s.ClusterManager, rs *reposerver.Server, hub *ws.Hub, cipher *crypto.Cipher) *Controller {
	return &Controller{
		cfg:            cfg,
		store:          st,
		k8s:            cm,
		repo:           rs,
		hub:            hub,
		cipher:         cipher,
		remoteClients:  make(map[string]*k8s.RemoteCluster),
		statusQ:        newWorkqueue(),
		syncQ:          newWorkqueue(),
		tracked:        make(map[string]struct{}),
		manualApplyGrace: make(map[string]time.Time),
	}
}

// Run blocks until ctx is cancelled, running reconcile + sync workers and
// the periodic resync ticker.
func (c *Controller) Run(ctx context.Context) error {
	slog.Info("controller starting",
		"reconcileWorkers", c.cfg.ReconcileWorkers,
		"resyncInterval", c.cfg.ReconcileResync)
	for i := 0; i < c.cfg.ReconcileWorkers; i++ {
		go c.statusWorker(ctx)
	}
	go c.syncWorker(ctx)
	go c.resyncTicker(ctx)
	go c.gitPollTicker(ctx)
	if c.cfg.AppsCatalogRepoURL != "" {
		go c.appsCatalogTicker(ctx)
	}
	c.enqueueAll(ctx)

	<-ctx.Done()
	c.statusQ.Close()
	c.syncQ.Close()
	return nil
}

// EnqueueStatus schedules a status reconcile for the given app name.
func (c *Controller) EnqueueStatus(appName string) { c.statusQ.Add(appName) }

// EnqueueSync schedules a sync execution.
func (c *Controller) EnqueueSync(appName string) { c.syncQ.Add(appName) }

// MarkManualApply records that a manual live-apply occurred for the given app,
// suppressing auto-sync for the configured grace period.
func (c *Controller) MarkManualApply(appName string) {
	c.manualApplyMu.Lock()
	defer c.manualApplyMu.Unlock()
	c.manualApplyGrace[appName] = time.Now()
}

// isManualApplyGrace checks whether the app is still within the manual-apply
// grace period during which auto-sync should be suppressed.
func (c *Controller) isManualApplyGrace(appName string) bool {
	c.manualApplyMu.Lock()
	defer c.manualApplyMu.Unlock()
	t, ok := c.manualApplyGrace[appName]
	if !ok {
		return false
	}
	if time.Since(t) > c.cfg.AutoSyncGracePeriod {
		delete(c.manualApplyGrace, appName)
		return false
	}
	return true
}

func (c *Controller) enqueueAll(ctx context.Context) {
	apps, err := c.store.Applications.List(ctx)
	if err != nil {
		slog.Error("enqueueAll: list apps", "err", err)
		return
	}
	for _, a := range apps {
		c.statusQ.Add(a.Name)
	}
}

func (c *Controller) resyncTicker(ctx context.Context) {
	t := time.NewTicker(c.cfg.ReconcileResync)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.enqueueAll(ctx)
		}
	}
}

func (c *Controller) gitPollTicker(ctx context.Context) {
	t := time.NewTicker(c.cfg.RepoPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.pollGit(ctx)
		}
	}
}

// pollGit resolves the head of every application's tracked ref; if it
// differs from the last observed revision, the app is re-queued.
func (c *Controller) pollGit(ctx context.Context) {
	apps, err := c.store.Applications.List(ctx)
	if err != nil {
		slog.Warn("git poll: list apps", "err", err)
		return
	}
	for _, a := range apps {
		sha, err := c.repo.ResolveRevision(ctx, a)
		if err != nil {
			slog.Debug("git poll: resolve", "app", a.Name, "err", err)
			continue
		}
		st, _ := c.store.Status.Get(ctx, a.ID)
		if st == nil || st.ObservedRevision != sha {
			c.statusQ.Add(a.Name)
		}
	}
}

func (c *Controller) statusWorker(ctx context.Context) {
	for {
		key, ok := c.statusQ.Get(ctx)
		if !ok {
			return
		}
		if err := c.reconcileStatus(ctx, key); err != nil {
			slog.Warn("reconcileStatus error", "app", key, "err", err)
			c.statusQ.AddAfter(key, 10*time.Second)
		}
		c.statusQ.Done(key)
	}
}

func (c *Controller) syncWorker(ctx context.Context) {
	for {
		key, ok := c.syncQ.Get(ctx)
		if !ok {
			return
		}
		if err := c.runSync(ctx, key); err != nil {
			slog.Warn("runSync error", "app", key, "err", err)
		}
		c.syncQ.Done(key)
	}
}

// reconcileStatus computes sync + health for one application and writes the
// result to application_status, publishing a WebSocket update on change.
func (c *Controller) reconcileStatus(ctx context.Context, appName string) error {
	app, err := c.store.Applications.GetByName(ctx, appName)
	if err != nil {
		return err
	}

	prev, _ := c.store.Status.Get(ctx, app.ID)
	newStatus := &domain.ApplicationStatus{AppID: app.ID}
	if prev != nil {
		newStatus.LastSyncedAt = prev.LastSyncedAt
	}

	rendered, err := c.repo.RenderForApp(ctx, app)
	if err != nil {
		newStatus.SyncStatus = domain.SyncStatusUnknown
		newStatus.HealthStatus = domain.HealthUnknown
		newStatus.Message = err.Error()
		_ = c.store.Status.Upsert(ctx, newStatus)
		c.publishStatus(app, newStatus)
		return err
	}
	newStatus.ObservedRevision = rendered.Revision

	kc, err := c.kubeClientForApp(ctx, app)
	if err != nil {
		newStatus.SyncStatus = domain.SyncStatusUnknown
		newStatus.HealthStatus = domain.HealthUnknown
		newStatus.Message = err.Error()
		_ = c.store.Status.Upsert(ctx, newStatus)
		c.publishStatus(app, newStatus)
		return err
	}

	applicable := manifest.FilterApplicable(rendered.Objects)
	live, healths, err := c.collectLive(ctx, app, applicable, kc)
	if err != nil {
		newStatus.SyncStatus = domain.SyncStatusUnknown
		newStatus.HealthStatus = domain.HealthUnknown
		newStatus.Message = err.Error()
		_ = c.store.Status.Upsert(ctx, newStatus)
		c.publishStatus(app, newStatus)
		return err
	}

	ds, err := k8s.Diff(applicable, live, app.SyncPolicy.IgnoreDifferences)
	if err != nil {
		newStatus.SyncStatus = domain.SyncStatusUnknown
		newStatus.Message = err.Error()
		_ = c.store.Status.Upsert(ctx, newStatus)
		c.publishStatus(app, newStatus)
		return err
	}
	newStatus.SyncStatus = domain.SyncStatusSynced
	if ds.OutOfSync > 0 {
		newStatus.SyncStatus = domain.SyncStatusOutOfSync
	}
	newStatus.HealthStatus = k8s.Aggregate(healths)

	if err := c.store.Status.Upsert(ctx, newStatus); err != nil {
		return err
	}
	c.publishStatus(app, newStatus)

	// Auto-sync hook: enqueue at most one unfinished sync per app. Without this
	// guard, every status reconcile while OutOfSync stacks Pending rows (resync
	// ticker, post-sync requeue, git poll) and looks like an infinite auto-sync.
	// Additionally, auto-sync is suppressed for a grace period after a manual
	// live-apply so the user's edit can persist and show as OutOfSync.
	if app.SyncPolicy.Automated != nil &&
		newStatus.SyncStatus == domain.SyncStatusOutOfSync &&
		!c.cfg.SyncDeniedAt(time.Now()) &&
		!c.isManualApplyGrace(app.Name) {
		busy, err := c.store.Sync.HasPendingOrRunning(ctx, app.ID)
		if err != nil {
			slog.Warn("auto-sync: check pending/running", "app", app.Name, "err", err)
			return nil
		}
		if busy {
			return nil
		}
		op := &domain.SyncOperation{
			AppID:       app.ID,
			Revision:    rendered.Revision,
			InitiatedBy: "auto-sync",
			Status:      domain.SyncOpPending,
		}
		_ = c.store.Sync.Create(ctx, op)
		c.EnqueueSync(app.Name)
	}
	c.reconcileChildResources(ctx, app, rendered.Objects)
	return nil
}

func (c *Controller) collectLive(ctx context.Context, app *domain.Application, desired []*unstructured.Unstructured, kc kubeClient) ([]*unstructured.Unstructured, []domain.HealthStatus, error) {
	gvrs := map[string]struct{}{}
	var live []*unstructured.Unstructured
	var healths []domain.HealthStatus
	for _, d := range desired {
		mapping, err := kc.MappingFor(d.GroupVersionKind())
		if err != nil {
			slog.Debug("collectLive: mapping miss", "gvk", d.GroupVersionKind(), "err", err)
			healths = append(healths, domain.HealthMissing)
			continue
		}
		if _, seen := gvrs[mapping.Resource.String()]; !seen {
			if err := kc.EnsureInformer(ctx, mapping.Resource); err != nil {
				return nil, nil, err
			}
			gvrs[mapping.Resource.String()] = struct{}{}
		}
		listNS := d.GetNamespace()
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if listNS == "" {
				listNS = app.DestNamespace
			}
		} else {
			listNS = ""
		}
		objs, err := kc.ListByLabel(ctx, mapping.Resource, listNS, labels.SelectorFromSet(labels.Set{manifest.TrackingLabel: app.Name}))
		if err != nil {
			return nil, nil, err
		}
		// Filter to the named object.
		var match *unstructured.Unstructured
		for _, o := range objs {
			if o.GetName() == d.GetName() {
				match = o
				break
			}
		}
		// Fallback: the object may exist but carry a different (or no) tracking
		// label (e.g. a Namespace created by another app or by hand). Fetch it
		// directly so the diff engine can compare desired vs live.
		if match == nil {
			if got, err := kc.GetByName(ctx, mapping.Resource, listNS, d.GetName()); err == nil {
				match = got
			}
		}
		if match == nil {
			healths = append(healths, domain.HealthMissing)
			continue
		}
		live = append(live, match)
		healths = append(healths, k8s.Health(match))
	}
	return live, healths, nil
}

func (c *Controller) publishStatus(app *domain.Application, st *domain.ApplicationStatus) {
	c.hub.Publish("app:"+app.Name+":status", "status", st)
}

func (c *Controller) kubeClientForApp(ctx context.Context, app *domain.Application) (kubeClient, error) {
	cl, err := c.store.Clusters.GetByID(ctx, app.DestClusterID)
	if err != nil {
		return nil, err
	}
	if cl.InCluster {
		return localClient{cm: c.k8s}, nil
	}
	if len(cl.AuthConfigEncrypted) == 0 {
		return nil, fmt.Errorf("cluster %q has no kubeconfig credentials", cl.Name)
	}
	plain, err := c.cipher.Decrypt(cl.AuthConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt cluster kubeconfig: %w", err)
	}
	yml := string(plain)

	c.remoteMu.Lock()
	defer c.remoteMu.Unlock()
	if rc, ok := c.remoteClients[cl.ID]; ok {
		return remoteClient{r: rc}, nil
	}
	rc, err := k8s.NewRemoteClusterFromKubeconfigYAML(yml)
	if err != nil {
		return nil, err
	}
	c.remoteClients[cl.ID] = rc
	return remoteClient{r: rc}, nil
}
