package api

import (
	"net/http"

	"github.com/orin/orin/internal/auth"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) createApplicationBatch(w http.ResponseWriter, r *http.Request) {
	var req apiv1.ApplicationBatchCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "empty_items", "items must not be empty")
		return
	}
	type rowResult struct {
		Name  string `json:"name"`
		Error string `json:"error,omitempty"`
	}
	var results []rowResult
	for _, item := range req.Items {
		if item.Name == "" {
			results = append(results, rowResult{Error: "missing item name"})
			continue
		}
		sub := req.Template
		sub.Name = item.Name
		if item.DestNamespace != "" {
			sub.Destination.Namespace = item.DestNamespace
		}
		if item.RepoURL != "" {
			sub.Source.RepoURL = item.RepoURL
		}
		if item.Path != "" {
			sub.Source.Path = item.Path
		}
		if item.TargetRevision != "" {
			sub.Source.TargetRevision = item.TargetRevision
		}
		if item.Cluster != "" {
			sub.Destination.Cluster = item.Cluster
		}
		if item.Project != "" {
			sub.Project = item.Project
		}
		if item.HelmValues != nil {
			sub.Source.HelmValues = item.HelmValues
		}
		if item.CreateNamespace != nil {
			sub.SyncPolicy.CreateNamespace = *item.CreateNamespace
		}
		rowProj := sub.Project
		if rowProj == "" {
			rowProj = "default"
		}
		if u, ok := auth.FromContext(r.Context()); !ok || !auth.CanAccessProject(u, rowProj) {
			results = append(results, rowResult{Name: item.Name, Error: "insufficient project scope"})
			continue
		}
		_, err := s.persistApplication(r.Context(), sub)
		if err != nil {
			if he, ok := err.(*httpCreateErr); ok {
				results = append(results, rowResult{Name: item.Name, Error: he.msg})
				continue
			}
			results = append(results, rowResult{Name: item.Name, Error: err.Error()})
			continue
		}
		s.opts.Controller.EnqueueStatus(item.Name)
		results = append(results, rowResult{Name: item.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
