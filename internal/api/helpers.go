package api

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"

	"github.com/k8s-ui/k8s-ui/internal/auth"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/store"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, apiv1.ErrorResponse{Error: errCode, Message: msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func notFoundOr500(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

func requireProjectAccess(w http.ResponseWriter, r *http.Request, project string) bool {
	u, ok := auth.FromContext(r.Context())
	if !ok || !auth.CanAccessProject(u, project) {
		writeError(w, http.StatusForbidden, "project_forbidden", "insufficient project scope")
		return false
	}
	return true
}

// toAPIApp converts a domain Application + status row into the public DTO.
func (s *Server) toAPIApp(ctx context.Context, app *domain.Application) (apiv1.Application, error) {
	repo, err := s.opts.Store.Repositories.GetByID(ctx, app.RepoID)
	if err != nil {
		return apiv1.Application{}, err
	}
	cl, err := s.opts.Store.Clusters.GetByID(ctx, app.DestClusterID)
	if err != nil {
		return apiv1.Application{}, err
	}
	st, _ := s.opts.Store.Status.Get(ctx, app.ID)
	src := apiv1.AppSource{
		RepoURL:        repo.URL,
		Path:           app.Path,
		TargetRevision: app.TargetRevision,
		HelmValueFiles: append([]string(nil), app.HelmValueFiles...),
	}
	if len(app.HelmValuesJSON) > 0 {
		src.HelmValues = append(json.RawMessage(nil), app.HelmValuesJSON...)
	}
	out := apiv1.Application{
		Name:    app.Name,
		Project: app.Project,
		Source:  src,
		Destination: apiv1.AppDestination{
			Cluster:   cl.Name,
			Namespace: app.DestNamespace,
		},
		CreatedAt: app.CreatedAt,
		UpdatedAt: app.UpdatedAt,
	}
	if app.SyncPolicy.Automated != nil {
		out.SyncPolicy.Automated = &apiv1.AutomatedSync{
			Prune:    app.SyncPolicy.Automated.Prune,
			SelfHeal: app.SyncPolicy.Automated.SelfHeal,
		}
	}
	out.SyncPolicy.CreateNamespace = app.SyncPolicy.CreateNamespace
	out.SyncPolicy.SyncOptions = append([]string(nil), app.SyncPolicy.SyncOptions...)
	if m := app.SyncPolicy.ManagedNamespaceMetadata; m != nil {
		out.SyncPolicy.ManagedNamespaceMetadata = &apiv1.ManagedNamespaceMetadata{
			Labels:      maps.Clone(m.Labels),
			Annotations: maps.Clone(m.Annotations),
		}
	}
	if len(app.SyncPolicy.IgnoreDifferences) > 0 {
		out.SyncPolicy.IgnoreDifferences = make([]apiv1.IgnoreDifferenceRule, len(app.SyncPolicy.IgnoreDifferences))
		for i, r := range app.SyncPolicy.IgnoreDifferences {
			out.SyncPolicy.IgnoreDifferences[i] = apiv1.IgnoreDifferenceRule{
				Group:        r.Group,
				Kind:         r.Kind,
				Name:         r.Name,
				Namespace:    r.Namespace,
				JSONPointers: append([]string(nil), r.JSONPointers...),
			}
		}
	}
	if st != nil {
		out.Status = apiv1.AppStatus{
			Sync:             string(st.SyncStatus),
			Health:           string(st.HealthStatus),
			ObservedRevision: st.ObservedRevision,
			LastSyncedAt:     st.LastSyncedAt,
			Message:          st.Message,
		}
		if st.ObservedRevision != "" {
			if ci, err := s.opts.Repo.CommitForRevision(ctx, app, st.ObservedRevision); err == nil && ci != nil {
				out.Status.ObservedCommit = &apiv1.GitCommit{
					SHA:        ci.SHA,
					ShortSHA:   ci.ShortSHA,
					Message:    ci.Message,
					Author:     ci.Author,
					AuthorDate: ci.AuthorDate,
				}
			}
		}
		ops, err := s.opts.Store.Sync.ListByApp(ctx, app.ID, 12)
		if err == nil {
			enrichAppStatusWithSyncOps(&out.Status, ops)
		}
	}
	return out, nil
}

func enrichAppStatusWithSyncOps(st *apiv1.AppStatus, ops []*domain.SyncOperation) {
	if len(ops) == 0 {
		return
	}
	var active *domain.SyncOperation
	for _, op := range ops {
		if op.Status == domain.SyncOpPending || op.Status == domain.SyncOpRunning {
			active = op
			break
		}
	}
	if active != nil {
		st.SyncOperation = &apiv1.SyncOperationProgress{
			Status:  string(active.Status),
			Message: active.Message,
		}
		return
	}
	for _, op := range ops {
		if op.FinishedAt != nil {
			st.LastCompletedSync = &apiv1.CompletedSyncSummary{
				Status:  string(op.Status),
				Message: op.Message,
			}
			break
		}
	}
}
