package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) appRevisions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	commits, err := s.opts.Repo.ListCommitsForApp(r.Context(), app, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "git_log_failed", err.Error())
		return
	}
	out := apiv1.RevisionListResponse{Commits: make([]apiv1.GitCommit, 0, len(commits))}
	for _, c := range commits {
		out.Commits = append(out.Commits, apiv1.GitCommit{
			SHA:        c.SHA,
			ShortSHA:   c.ShortSHA,
			Message:    c.Message,
			Author:     c.Author,
			AuthorDate: c.AuthorDate,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) appRevisionDiff(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "missing_params", "from and to query params (commit SHAs) are required")
		return
	}
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	diff, err := s.opts.Repo.DiffGitPaths(r.Context(), app, from, to)
	if err != nil {
		writeError(w, http.StatusBadGateway, "git_diff_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiv1.RevisionDiffResponse{Diff: diff})
}

func (s *Server) appRollback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req apiv1.RollbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Revision == "" {
		writeError(w, http.StatusBadRequest, "missing_revision", "revision is required")
		return
	}
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	app.TargetRevision = req.Revision
	if err := s.opts.Store.Applications.Update(r.Context(), app); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(app.Name)
	writeJSON(w, http.StatusOK, map[string]string{"targetRevision": app.TargetRevision})
}
