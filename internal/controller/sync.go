package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orin/orin/internal/config"
	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/k8s"
	"github.com/orin/orin/internal/manifest"
	"github.com/orin/orin/internal/metrics"
	"github.com/orin/orin/internal/notify"
)

// runSync executes the next pending SyncOperation for the application.
func (c *Controller) runSync(ctx context.Context, appName string) error {
	app, err := c.store.Applications.GetByName(ctx, appName)
	if err != nil {
		return err
	}
	// Atomically claim the next pending operation using SELECT FOR UPDATE SKIP LOCKED.
	// This prevents multiple controller pods from claiming the same operation.
	op, err := c.store.Sync.ClaimNextPending(ctx, app.ID)
	if err != nil {
		return err
	}
	if op == nil {
		return nil
	}
	c.publishSyncEvent(app, op)

	if c.cfg.SyncDeniedAt(time.Now()) {
		c.finishOp(ctx, app, op, domain.SyncOpFailed, "sync denied: maintenance window (SYNC_DENY_RANGE_UTC)")
		return nil
	}

	kc, err := c.kubeClientForApp(ctx, app)
	if err != nil {
		c.finishOp(ctx, app, op, domain.SyncOpFailed, fmt.Sprintf("cluster client: %v", err))
		return err
	}

	rendered, err := c.repo.RenderForApp(ctx, app)
	if err != nil {
		c.finishOp(ctx, app, op, domain.SyncOpFailed, fmt.Sprintf("render: %v", err))
		return err
	}

	// Strip control-plane objects (orin.io/* and argoproj.io Application/AppProject)
	// before applying: these declare child resources in the DB but must never be
	// sent to the destination Kubernetes cluster.
	applicable := manifest.FilterApplicable(rendered.Objects)
	toApply := filterRenderedObjects(applicable, op.Request.Resources)
	sortForApply(toApply)
	if len(toApply) == 0 {
		if len(op.Request.Resources) > 0 {
			c.finishOp(ctx, app, op, domain.SyncOpFailed, "no rendered objects matched the selected resources")
			st := &domain.ApplicationStatus{
				AppID:            app.ID,
				ObservedRevision: rendered.Revision,
				SyncStatus:       domain.SyncStatusOutOfSync,
				HealthStatus:     domain.HealthUnknown,
				Message:          op.Message,
			}
			_ = c.store.Status.Upsert(ctx, st)
			c.publishStatus(app, st)
			c.statusQ.AddAfter(app.Name, 5*time.Second)
			return nil
		}
	}

	dry := op.Request.DryRun

	// Run PreSync hooks before applying main resources
	if !dry {
		if err := c.executeHooks(ctx, app, domain.HookPreSync, op, kc); err != nil {
			c.finishOp(ctx, app, op, domain.SyncOpFailed, fmt.Sprintf("pre-sync hook failed: %v", err))
			_ = c.runSyncFailHooks(ctx, app, op, kc)
			return err
		}
	}

	if app.SyncPolicy.EffectiveCreateNamespace() && app.DestNamespace != "" {
		meta := map[string]interface{}{
			"name": app.DestNamespace,
		}
		if m := app.SyncPolicy.ManagedNamespaceMetadata; m != nil {
			if len(m.Labels) > 0 {
				meta["labels"] = stringMapToUnstructured(m.Labels)
			}
			if len(m.Annotations) > 0 {
				meta["annotations"] = stringMapToUnstructured(m.Annotations)
			}
		}
		nsObj := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   meta,
		}}
		res := applyOneWithRetry(ctx, c.cfg, kc, nsObj, dry)
		op.Resources = append(op.Resources, res)
		if err := c.store.Sync.Update(ctx, op); err != nil {
			slog.Warn("sync progress update after namespace", "err", err)
		}
	}
	for _, obj := range toApply {
		res := applyOneWithRetry(ctx, c.cfg, kc, obj, dry)
		op.Resources = append(op.Resources, res)
		if err := c.store.Sync.Update(ctx, op); err != nil {
			slog.Warn("sync progress update failed", "err", err)
		}
	}

	// Prune only on full sync (all manifests), never on partial selection; skip entirely in dry-run.
	fullSync := len(op.Request.Resources) == 0
	if fullSync && !dry {
		if op.Request.Prune || (app.SyncPolicy.Automated != nil && app.SyncPolicy.Automated.Prune) {
			c.pruneRemoved(ctx, app, applicable, op, kc)
			if err := c.store.Sync.Update(ctx, op); err != nil {
				slog.Warn("sync progress update failed after prune", "err", err)
			}
		}
	}

	finalStatus := computeFinalStatus(op)
	opMsg := syncPartialFailureMessage(op, finalStatus)

	// Run PostSync hooks after successful sync
	if finalStatus == domain.SyncOpSucceeded && !op.Request.DryRun {
		if err := c.executeHooks(ctx, app, domain.HookPostSync, op, kc); err != nil {
			opMsg = fmt.Sprintf("post-sync hook failed: %v", err)
			finalStatus = domain.SyncOpFailed
		}
	}

	// Run SyncFail hooks on sync failure
	if finalStatus == domain.SyncOpFailed && !op.Request.DryRun {
		_ = c.runSyncFailHooks(ctx, app, op, kc)
	}

	c.finishOp(ctx, app, op, finalStatus, opMsg)

	// Immediately compute health from live resources instead of defaulting to Progressing
	_, healths, err := c.collectLive(ctx, app, applicable, kc)
	var healthStatus domain.HealthStatus
	if err != nil {
		healthStatus = domain.HealthUnknown
	} else {
		healthStatus = k8s.Aggregate(healths)
	}

	now := time.Now().UTC()
	st := &domain.ApplicationStatus{
		AppID:            app.ID,
		ObservedRevision: rendered.Revision,
		LastSyncedAt:     &now,
		SyncStatus:       domain.SyncStatusSynced,
		HealthStatus:     healthStatus,
	}
	if op.Status != domain.SyncOpSucceeded {
		st.SyncStatus = domain.SyncStatusOutOfSync
		st.Message = op.Message
	} else if opMsg != "" {
		st.Message = opMsg
	}
	_ = c.store.Status.Upsert(ctx, st)
	c.publishStatus(app, st)
	// Re-evaluate health shortly after a sync to catch any changes
	c.statusQ.AddAfter(app.Name, 5*time.Second)
	return nil
}

func (c *Controller) finishOp(ctx context.Context, app *domain.Application, op *domain.SyncOperation, status domain.SyncOpStatus, msg string) {
	now := time.Now().UTC()
	op.FinishedAt = &now
	op.Status = status
	if msg != "" {
		op.Message = msg
	}
	_ = c.store.Sync.Update(ctx, op)
	c.publishSyncEvent(app, op)
	metrics.SyncOperations.WithLabelValues(string(status)).Inc()

	// Dispatch notifications for terminal sync states
	if c.notifier != nil && (status == domain.SyncOpSucceeded || status == domain.SyncOpFailed) {
		event := domain.EventSyncFailed
		if status == domain.SyncOpSucceeded {
			event = domain.EventSyncSucceeded
		}
		c.dispatchNotification(ctx, app, event, notify.Payload{
			AppName:     app.Name,
			Event:       string(event),
			Status:      string(status),
			Message:     msg,
			InitiatedBy: op.InitiatedBy,
			Revision:    op.Revision,
			Timestamp:   now.Format(time.RFC3339),
		})
	}
}

func (c *Controller) dispatchNotification(ctx context.Context, app *domain.Application, event domain.NotificationEventType, p notify.Payload) {
	// App-specific configs
	configs, err := c.store.Notifications.ListEnabledForEvent(ctx, app.ID, event)
	if err != nil {
		slog.Debug("notify: list configs", "app", app.Name, "err", err)
	}
	for _, cfg := range configs {
		c.notifier.Send(ctx, cfg, p)
	}
	// Global configs (app_id = '*')
	globalConfigs, err := c.store.Notifications.ListEnabledForEvent(ctx, "*", event)
	if err != nil {
		slog.Debug("notify: list global configs", "err", err)
	}
	for _, cfg := range globalConfigs {
		c.notifier.Send(ctx, cfg, p)
	}
}

func (c *Controller) publishSyncEvent(app *domain.Application, op *domain.SyncOperation) {
	c.hub.Publish("app:"+app.Name+":sync", "sync", op)
}

func computeFinalStatus(op *domain.SyncOperation) domain.SyncOpStatus {
	if len(op.Resources) == 0 {
		return domain.SyncOpSucceeded
	}
	applied := 0
	for _, r := range op.Resources {
		if r.Status == "Applied" || r.Status == "Pruned" || r.Status == "DryRun" {
			applied++
		}
	}
	// If at least one object was applied, treat the sync as succeeded so later
	// objects still get applied and the UI can show live resources; failures are
	// recorded per resource in op.Resources.
	if applied > 0 {
		return domain.SyncOpSucceeded
	}
	return domain.SyncOpFailed
}

func syncPartialFailureMessage(op *domain.SyncOperation, st domain.SyncOpStatus) string {
	if st != domain.SyncOpSucceeded {
		return ""
	}
	var failed []string
	for _, r := range op.Resources {
		if r.Status != "Failed" {
			continue
		}
		// Truncate long error messages
		msg := r.Message
		if len(msg) > 100 {
			msg = msg[:97] + "..."
		}
		failed = append(failed, fmt.Sprintf("%s/%s: %s", r.Kind, r.Name, msg))
	}
	if len(failed) == 0 {
		return ""
	}

	// If there are many failures, show only first 3 and count
	if len(failed) > 3 {
		return fmt.Sprintf("%d resources failed. First 3: %s", len(failed), strings.Join(failed[:3], "; "))
	}
	return "Some resources failed: " + strings.Join(failed, "; ")
}

func filterRenderedObjects(objects []*unstructured.Unstructured, keys []string) []*unstructured.Unstructured {
	if len(keys) == 0 {
		return objects
	}
	want := map[string]struct{}{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			want[k] = struct{}{}
		}
	}
	if len(want) == 0 {
		return objects
	}
	var out []*unstructured.Unstructured
	for _, obj := range objects {
		if _, ok := want[key(obj)]; ok {
			out = append(out, obj)
		}
	}
	return out
}

func applyOneWithRetry(ctx context.Context, cfg *config.Config, kc kubeClient, obj *unstructured.Unstructured, dryRun bool) domain.SyncResourceResult {
	attempts := cfg.SyncApplyRetries
	if attempts < 1 {
		attempts = 1
	}
	var last domain.SyncResourceResult
	for i := 0; i < attempts; i++ {
		last = applyOne(ctx, kc, obj, dryRun)
		if last.Status != "Failed" {
			return last
		}
		if i < attempts-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
	return last
}

func applyOne(ctx context.Context, kc kubeClient, obj *unstructured.Unstructured, dryRun bool) domain.SyncResourceResult {
	res := domain.SyncResourceResult{
		Group:     obj.GroupVersionKind().Group,
		Version:   obj.GroupVersionKind().Version,
		Kind:      obj.GetKind(),
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Status:    "Applied",
	}
	if dryRun {
		res.Status = "DryRun"
	}
	if _, err := kc.Apply(ctx, obj, k8s.ApplyOptions{Force: true, DryRun: dryRun}); err != nil {
		res.Status = "Failed"
		res.Message = err.Error()
		slog.Warn("apply failed", "obj", obj.GetName(), "err", err)
	}
	return res
}

func (c *Controller) pruneRemoved(ctx context.Context, app *domain.Application, desired []*unstructured.Unstructured, op *domain.SyncOperation, kc kubeClient) {
	desiredKeys := map[string]struct{}{}
	for _, d := range desired {
		desiredKeys[key(d)] = struct{}{}
	}
	for _, gvr := range pruneCandidateGVRs() {
		objs, err := kc.ListByLabel(ctx, gvr, app.DestNamespace, trackingSelector(app.Name))
		if err != nil {
			continue
		}
		for _, o := range objs {
			if _, ok := desiredKeys[key(o)]; ok {
				continue
			}
			err := kc.Delete(ctx, o.GroupVersionKind(), o.GetNamespace(), o.GetName())
			res := domain.SyncResourceResult{
				Group: o.GroupVersionKind().Group, Version: o.GroupVersionKind().Version,
				Kind: o.GetKind(), Namespace: o.GetNamespace(), Name: o.GetName(),
				Status: "Pruned",
			}
			if err != nil {
				res.Status = "Failed"
				res.Message = err.Error()
			}
			op.Resources = append(op.Resources, res)
		}
	}
}

func key(o *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s/%s", o.GroupVersionKind().Group, o.GetKind(), o.GetNamespace(), o.GetName())
}

func stringMapToUnstructured(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// executeHooks applies hook manifests (Job/Pod) and waits for completion.
func (c *Controller) executeHooks(ctx context.Context, app *domain.Application, phase domain.SyncHookPhase, op *domain.SyncOperation, kc kubeClient) error {
	hooks, err := c.store.SyncHooks.ListByPhase(ctx, app.ID, phase)
	if err != nil {
		slog.Warn("hooks: list", "app", app.Name, "phase", phase, "err", err)
		return nil // non-fatal: don't block sync on hook DB errors
	}
	for _, h := range hooks {
		obj, err := parseYAMLToUnstructured(h.YAML)
		if err != nil {
			slog.Warn("hooks: parse", "hook", h.Name, "err", err)
			continue
		}
		slog.Info("hooks: applying", "hook", h.Name, "phase", phase)
		res := applyOneWithRetry(ctx, c.cfg, kc, obj, false)
		res.Kind = "Hook:" + string(phase)
		op.Resources = append(op.Resources, res)
		if res.Status == "Failed" {
			if err := c.store.Sync.Update(ctx, op); err != nil {
				slog.Warn("hooks: update progress", "err", err)
			}
			return fmt.Errorf("hook %s (%s) failed: %s", h.Name, phase, res.Message)
		}
		slog.Info("hooks: applied", "hook", h.Name, "phase", phase)
		if err := c.store.Sync.Update(ctx, op); err != nil {
			slog.Warn("hooks: update progress", "err", err)
		}
	}
	return nil
}

func parseYAMLToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	return manifest.ParseYAMLToUnstructured([]byte(yamlStr))
}

// runSyncFailHooks applies SyncFail hooks. Their failure is logged but doesn't change the already-failed status.
func (c *Controller) runSyncFailHooks(ctx context.Context, app *domain.Application, op *domain.SyncOperation, kc kubeClient) error {
	return c.executeHooks(ctx, app, domain.HookSyncFail, op, kc)
}
