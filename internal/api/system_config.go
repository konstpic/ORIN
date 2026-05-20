package api

import (
	"net/http"
	"time"

	"github.com/orin/orin/internal/config"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) getSystemConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.opts.Store.SystemConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_failed", err.Error())
		return
	}

	envCfg := s.opts.Config
	resp := apiv1.SystemConfigResponse{
		ReconcileWorkers:    cfg.ReconcileWorkers,
		ReconcileResync:     cfg.ReconcileResync.String(),
		RepoPollInterval:    cfg.RepoPollInterval.String(),
		RepoRenderTimeout:   cfg.RepoRenderTimeout.String(),
		SyncApplyRetries:    cfg.SyncApplyRetries,
		AutoSyncGracePeriod: cfg.AutoSyncGracePeriod.String(),
		SyncDenyRangeUtc:    cfg.SyncDenyRangeUTC,
		AppsCatalogRepoUrl:  cfg.AppsCatalogRepoURL,
		AppsCatalogPath:     cfg.AppsCatalogPath,
		AppsCatalogInterval: cfg.AppsCatalogInterval.String(),
	}

	// Fall back to env defaults for zero-values
	if resp.ReconcileWorkers == 0 {
		resp.ReconcileWorkers = envCfg.ReconcileWorkers
	}
	if resp.ReconcileResync == "0s" {
		resp.ReconcileResync = envCfg.ReconcileResync.String()
	}
	if resp.RepoPollInterval == "0s" {
		resp.RepoPollInterval = envCfg.RepoPollInterval.String()
	}
	if resp.RepoRenderTimeout == "0s" {
		resp.RepoRenderTimeout = envCfg.RepoRenderTimeout.String()
	}
	if resp.SyncApplyRetries == 0 {
		resp.SyncApplyRetries = envCfg.SyncApplyRetries
	}
	if resp.AutoSyncGracePeriod == "0s" {
		resp.AutoSyncGracePeriod = envCfg.AutoSyncGracePeriod.String()
	}
	if resp.AppsCatalogPath == "" {
		resp.AppsCatalogPath = envCfg.AppsCatalogPath
	}
	if resp.AppsCatalogInterval == "0s" {
		resp.AppsCatalogInterval = envCfg.AppsCatalogInterval.String()
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) updateSystemConfig(w http.ResponseWriter, r *http.Request) {
	var req apiv1.UpdateSystemConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	// Read current config first
	current, err := s.opts.Store.SystemConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_failed", err.Error())
		return
	}

	// Apply updates (only non-nil fields)
	if req.ReconcileWorkers != nil {
		current.ReconcileWorkers = *req.ReconcileWorkers
	}
	if req.ReconcileResync != nil {
		d, err := time.ParseDuration(*req.ReconcileResync)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_duration", "reconcileResync: "+err.Error())
			return
		}
		current.ReconcileResync = d
	}
	if req.RepoPollInterval != nil {
		d, err := time.ParseDuration(*req.RepoPollInterval)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_duration", "repoPollInterval: "+err.Error())
			return
		}
		current.RepoPollInterval = d
	}
	if req.RepoRenderTimeout != nil {
		d, err := time.ParseDuration(*req.RepoRenderTimeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_duration", "repoRenderTimeout: "+err.Error())
			return
		}
		current.RepoRenderTimeout = d
	}
	if req.SyncApplyRetries != nil {
		current.SyncApplyRetries = *req.SyncApplyRetries
	}
	if req.AutoSyncGracePeriod != nil {
		d, err := time.ParseDuration(*req.AutoSyncGracePeriod)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_duration", "autoSyncGracePeriod: "+err.Error())
			return
		}
		current.AutoSyncGracePeriod = d
	}
	if req.SyncDenyRangeUtc != nil {
		// Validate format
		_, err := config.ParseSyncDenyRangeUTC(*req.SyncDenyRangeUtc)
		if err != nil && *req.SyncDenyRangeUtc != "" {
			writeError(w, http.StatusBadRequest, "invalid_sync_deny_range", err.Error())
			return
		}
		current.SyncDenyRangeUTC = *req.SyncDenyRangeUtc
	}
	if req.AppsCatalogRepoUrl != nil {
		current.AppsCatalogRepoURL = *req.AppsCatalogRepoUrl
	}
	if req.AppsCatalogPath != nil {
		current.AppsCatalogPath = *req.AppsCatalogPath
	}
	if req.AppsCatalogInterval != nil {
		d, err := time.ParseDuration(*req.AppsCatalogInterval)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_duration", "appsCatalogInterval: "+err.Error())
			return
		}
		current.AppsCatalogInterval = d
	}

	if err := s.opts.Store.SystemConfig.Update(r.Context(), current); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	// Return updated config
	s.getSystemConfig(w, r)
}
