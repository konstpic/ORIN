package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orin/orin/internal/domain"
)

// SystemConfigStore reads and writes system configuration key-value pairs.
type SystemConfigStore struct {
	pool *pgxpool.Pool
}

func NewSystemConfigStore(pool *pgxpool.Pool) *SystemConfigStore {
	return &SystemConfigStore{pool: pool}
}

func (s *SystemConfigStore) Get(ctx context.Context) (*domain.SystemConfig, error) {
	cfg := &domain.SystemConfig{}
	rows, err := s.pool.Query(ctx, "SELECT key, value FROM system_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		switch key {
		case "reconcile_workers":
			cfg.ReconcileWorkers, _ = strconv.Atoi(value)
		case "reconcile_resync":
			cfg.ReconcileResync, _ = time.ParseDuration(value)
		case "repo_poll_interval":
			cfg.RepoPollInterval, _ = time.ParseDuration(value)
		case "repo_render_timeout":
			cfg.RepoRenderTimeout, _ = time.ParseDuration(value)
		case "sync_apply_retries":
			cfg.SyncApplyRetries, _ = strconv.Atoi(value)
		case "auto_sync_grace_period":
			cfg.AutoSyncGracePeriod, _ = time.ParseDuration(value)
		case "sync_deny_range_utc":
			cfg.SyncDenyRangeUTC = value
		case "apps_catalog_repo_url":
			cfg.AppsCatalogRepoURL = value
		case "apps_catalog_path":
			cfg.AppsCatalogPath = value
		case "apps_catalog_interval":
			cfg.AppsCatalogInterval, _ = time.ParseDuration(value)
		}
	}
	return cfg, nil
}

func (s *SystemConfigStore) Update(ctx context.Context, cfg *domain.SystemConfig) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	settings := []struct {
		key   string
		value string
	}{
		{"reconcile_workers", strconv.Itoa(cfg.ReconcileWorkers)},
		{"reconcile_resync", cfg.ReconcileResync.String()},
		{"repo_poll_interval", cfg.RepoPollInterval.String()},
		{"repo_render_timeout", cfg.RepoRenderTimeout.String()},
		{"sync_apply_retries", strconv.Itoa(cfg.SyncApplyRetries)},
		{"auto_sync_grace_period", cfg.AutoSyncGracePeriod.String()},
		{"sync_deny_range_utc", cfg.SyncDenyRangeUTC},
		{"apps_catalog_repo_url", cfg.AppsCatalogRepoURL},
		{"apps_catalog_path", cfg.AppsCatalogPath},
		{"apps_catalog_interval", cfg.AppsCatalogInterval.String()},
	}

	for _, s := range settings {
		_, err := tx.Exec(ctx,
			"INSERT INTO system_config (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()",
			s.key, s.value)
		if err != nil {
			return fmt.Errorf("setting %s: %w", s.key, err)
		}
	}

	return tx.Commit(ctx)
}
