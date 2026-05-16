package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

func (s *Server) listSyncHooks(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	hooks, err := s.opts.Store.SyncHooks.ListByApp(r.Context(), app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.SyncHook, 0, len(hooks))
	for _, h := range hooks {
		out = append(out, hookToAPI(h))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createSyncHook(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	var req apiv1.CreateSyncHookRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	h := &domain.SyncHook{
		ID:      uuid.NewString(),
		AppID:   app.ID,
		Name:    req.Name,
		Phase:   domain.SyncHookPhase(req.Phase),
		YAML:    req.YAML,
		Enabled: req.Enabled,
	}
	if err := s.opts.Store.SyncHooks.Create(r.Context(), h); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, hookToAPI(h))
}

func (s *Server) updateSyncHook(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	id := chi.URLParam(r, "hookId")
	var req apiv1.UpdateSyncHookRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	h, err := s.opts.Store.SyncHooks.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if h.AppID != app.ID {
		writeError(w, http.StatusForbidden, "forbidden", "hook does not belong to this application")
		return
	}
	h.Name = req.Name
	h.Phase = domain.SyncHookPhase(req.Phase)
	h.YAML = req.YAML
	h.Enabled = req.Enabled
	if err := s.opts.Store.SyncHooks.Update(r.Context(), h); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hookToAPI(h))
}

func (s *Server) deleteSyncHook(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	id := chi.URLParam(r, "hookId")
	h, err := s.opts.Store.SyncHooks.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if h.AppID != app.ID {
		writeError(w, http.StatusForbidden, "forbidden", "hook does not belong to this application")
		return
	}
	if err := s.opts.Store.SyncHooks.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hookToAPI(h *domain.SyncHook) apiv1.SyncHook {
	return apiv1.SyncHook{
		ID:        h.ID,
		AppID:     h.AppID,
		Name:      h.Name,
		Phase:     string(h.Phase),
		YAML:      h.YAML,
		Enabled:   h.Enabled,
		CreatedAt: h.CreatedAt,
		UpdatedAt: h.UpdatedAt,
	}
}
