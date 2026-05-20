package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/k8s"
	"github.com/orin/orin/internal/manifest"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func resourceAPIKey(group, kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s/%s", group, kind, namespace, name)
}

func desiredKeysFromDiff(ds *k8s.DiffSet) map[string]struct{} {
	m := make(map[string]struct{})
	for _, it := range ds.Items {
		m[resourceAPIKey(it.Group, it.Kind, it.Namespace, it.Name)] = struct{}{}
	}
	return m
}

func syncStatusByKey(ds *k8s.DiffSet) map[string]string {
	m := make(map[string]string)
	for _, it := range ds.Items {
		k := resourceAPIKey(it.Group, it.Kind, it.Namespace, it.Name)
		if it.Synced {
			m[k] = "Synced"
		} else {
			m[k] = "OutOfSync"
		}
	}
	return m
}

func latestActiveSyncOp(ctx context.Context, s *Server, appID string) *domain.SyncOperation {
	ops, err := s.opts.Store.Sync.ListByApp(ctx, appID, 12)
	if err != nil || len(ops) == 0 {
		return nil
	}
	for _, op := range ops {
		if op.Status == domain.SyncOpPending || op.Status == domain.SyncOpRunning {
			return op
		}
	}
	return nil
}

func mergeResourceNode(n *apiv1.ResourceNode, syncByKey map[string]string, desiredKeys map[string]struct{}, active *domain.SyncOperation) {
	k := resourceAPIKey(n.Group, n.Kind, n.Namespace, n.Name)
	if s, ok := syncByKey[k]; ok {
		n.Sync = s
	} else {
		n.Sync = "Synced"
	}
	n.SyncMessage = ""
	if active != nil && (active.Status == domain.SyncOpPending || active.Status == domain.SyncOpRunning) {
		done := make(map[string]domain.SyncResourceResult)
		for _, rr := range active.Resources {
			done[resourceAPIKey(rr.Group, rr.Kind, rr.Namespace, rr.Name)] = rr
		}
		if rr, ok := done[k]; ok {
			n.SyncMessage = rr.Message
			switch rr.Status {
			case "Failed":
				n.Sync = "OutOfSync"
			case "Applied", "DryRun", "Pruned":
				n.Sync = "Synced"
			}
		} else if _, want := desiredKeys[k]; want {
			n.Health = "Progressing"
		}
	}
	for i := range n.Children {
		mergeResourceNode(&n.Children[i], syncByKey, desiredKeys, active)
	}
}

func (s *Server) enrichResourceTree(r *http.Request, app *domain.Application, out *apiv1.ResourceTree) {
	res, err := s.opts.Repo.RenderForApp(r.Context(), app)
	if err != nil {
		// If we can't render, mark all resources as Unknown sync status
		for i := range out.Nodes {
			setUnknownSyncRecursive(&out.Nodes[i])
		}
		return
	}
	applicable := manifest.FilterApplicable(res.Objects)
	live, _, err := collectLiveForAPI(r, s, app, applicable)
	if err != nil {
		// If we can't get live resources, mark as Unknown
		for i := range out.Nodes {
			setUnknownSyncRecursive(&out.Nodes[i])
		}
		return
	}
	ds, err := k8s.Diff(applicable, live, app.SyncPolicy.IgnoreDifferences)
	if err != nil {
		// If diff fails, mark as Unknown
		for i := range out.Nodes {
			setUnknownSyncRecursive(&out.Nodes[i])
		}
		return
	}
	syncByKey := syncStatusByKey(ds)
	desiredKeys := desiredKeysFromDiff(ds)
	active := latestActiveSyncOp(r.Context(), s, app.ID)
	for i := range out.Nodes {
		mergeResourceNode(&out.Nodes[i], syncByKey, desiredKeys, active)
	}
	if active != nil {
		out.ActiveSync = &apiv1.ActiveSyncInfo{
			ID:        active.ID,
			Status:    string(active.Status),
			Message:   active.Message,
			Resources: toAPISyncResults(active.Resources),
		}
	}
}

func setUnknownSyncRecursive(n *apiv1.ResourceNode) {
	n.Sync = "Unknown"
	for i := range n.Children {
		setUnknownSyncRecursive(&n.Children[i])
	}
}
