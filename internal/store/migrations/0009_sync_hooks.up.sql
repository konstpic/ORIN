CREATE TABLE IF NOT EXISTS sync_hooks (
    id          TEXT PRIMARY KEY,
    app_id      TEXT REFERENCES applications(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    phase       TEXT NOT NULL CHECK (phase IN ('PreSync', 'PostSync', 'SyncFail')),
    yaml        TEXT NOT NULL,  -- the Kubernetes manifest (Job/Pod)
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_hooks_app ON sync_hooks(app_id);
CREATE INDEX IF NOT EXISTS idx_sync_hooks_phase ON sync_hooks(app_id, phase, enabled);
