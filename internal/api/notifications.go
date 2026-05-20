package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orin/orin/internal/domain"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) listNotificationConfigs(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	configs, err := s.opts.Store.Notifications.ListByApp(r.Context(), app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.NotificationConfig, 0, len(configs))
	for _, c := range configs {
		out = append(out, cfgToAPI(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createNotificationConfig(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	var req apiv1.CreateNotificationConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	cfg := &domain.NotificationConfig{
		ID:        uuid.NewString(),
		AppID:     app.ID,
		Name:      req.Name,
		Type:      domain.NotificationType(req.Type),
		URL:       req.URL,
		Enabled:   req.Enabled,
		CreatedAt: time.Now().UTC(),
	}
	for _, e := range req.Events {
		cfg.Events = append(cfg.Events, domain.NotificationEventType(e))
	}
	if err := s.opts.Store.Notifications.Create(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cfgToAPI(cfg))
}

func (s *Server) updateNotificationConfig(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	id := chi.URLParam(r, "configId")
	var req apiv1.UpdateNotificationConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	cfg, err := s.opts.Store.Notifications.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if cfg.AppID != app.ID {
		writeError(w, http.StatusForbidden, "forbidden", "config does not belong to this application")
		return
	}
	cfg.Name = req.Name
	cfg.Type = domain.NotificationType(req.Type)
	cfg.URL = req.URL
	cfg.Enabled = req.Enabled
	cfg.Events = nil
	for _, e := range req.Events {
		cfg.Events = append(cfg.Events, domain.NotificationEventType(e))
	}
	if err := s.opts.Store.Notifications.Update(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfgToAPI(cfg))
}

func (s *Server) deleteNotificationConfig(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}
	id := chi.URLParam(r, "configId")
	cfg, err := s.opts.Store.Notifications.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if cfg.AppID != app.ID {
		writeError(w, http.StatusForbidden, "forbidden", "config does not belong to this application")
		return
	}
	if err := s.opts.Store.Notifications.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postNotificationTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "url is required")
		return
	}
	if err := s.opts.Notifier.Test(r.Context(), req.URL); err != nil {
		writeError(w, http.StatusBadGateway, "test_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "test notification sent successfully"})
}

func cfgToAPI(c *domain.NotificationConfig) apiv1.NotificationConfig {
	events := make([]string, len(c.Events))
	for i, e := range c.Events {
		events[i] = string(e)
	}
	return apiv1.NotificationConfig{
		ID:        c.ID,
		AppID:     c.AppID,
		Name:      c.Name,
		Type:      string(c.Type),
		URL:       c.URL,
		Events:    events,
		Enabled:   c.Enabled,
		CreatedAt: c.CreatedAt,
	}
}

// --- Global notifications (app_id = '*' means all apps) ---

const globalAppID = "*"

func (s *Server) listGlobalNotificationConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := s.opts.Store.Notifications.ListByApp(r.Context(), globalAppID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.NotificationConfig, 0, len(configs))
	for _, c := range configs {
		out = append(out, cfgToAPI(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createGlobalNotificationConfig(w http.ResponseWriter, r *http.Request) {
	var req apiv1.CreateNotificationConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	cfg := &domain.NotificationConfig{
		ID:        uuid.NewString(),
		AppID:     globalAppID,
		Name:      req.Name,
		Type:      domain.NotificationType(req.Type),
		URL:       req.URL,
		Enabled:   req.Enabled,
		CreatedAt: time.Now().UTC(),
	}
	for _, e := range req.Events {
		cfg.Events = append(cfg.Events, domain.NotificationEventType(e))
	}
	if err := s.opts.Store.Notifications.Create(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cfgToAPI(cfg))
}

func (s *Server) updateGlobalNotificationConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "configId")
	var req apiv1.UpdateNotificationConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	cfg, err := s.opts.Store.Notifications.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if cfg.AppID != globalAppID {
		writeError(w, http.StatusBadRequest, "not_global", "this config is app-specific, not editable here")
		return
	}
	cfg.Name = req.Name
	cfg.Type = domain.NotificationType(req.Type)
	cfg.URL = req.URL
	cfg.Enabled = req.Enabled
	cfg.Events = nil
	for _, e := range req.Events {
		cfg.Events = append(cfg.Events, domain.NotificationEventType(e))
	}
	if err := s.opts.Store.Notifications.Update(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfgToAPI(cfg))
}

func (s *Server) deleteGlobalNotificationConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "configId")
	cfg, err := s.opts.Store.Notifications.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if cfg.AppID != globalAppID {
		writeError(w, http.StatusBadRequest, "not_global", "this config is app-specific")
		return
	}
	if err := s.opts.Store.Notifications.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
