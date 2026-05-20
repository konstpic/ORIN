-- System configuration: runtime-overridable settings stored in DB.
-- These values supplement environment variables and can be changed via API/UI.

CREATE TABLE IF NOT EXISTS system_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed with defaults that mirror env-based config
INSERT INTO system_config (key, value) VALUES
    ('reconcile_workers', '10'),
    ('reconcile_resync', '3m'),
    ('repo_poll_interval', '3m'),
    ('repo_render_timeout', '60s'),
    ('sync_apply_retries', '1'),
    ('auto_sync_grace_period', '30m'),
    ('sync_deny_range_utc', ''),
    ('apps_catalog_repo_url', ''),
    ('apps_catalog_path', 'orin/apps.yaml'),
    ('apps_catalog_interval', '5m')
ON CONFLICT (key) DO NOTHING;
