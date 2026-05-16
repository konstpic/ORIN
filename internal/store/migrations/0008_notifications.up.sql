CREATE TABLE IF NOT EXISTS notification_configs (
    id          TEXT PRIMARY KEY,
    app_id      TEXT REFERENCES applications(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('webhook', 'slack')),
    url         TEXT NOT NULL,
    events      TEXT NOT NULL DEFAULT '[]',  -- JSON array: ["sync_failed","sync_succeeded","health_degraded"]
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_configs_app ON notification_configs(app_id);
