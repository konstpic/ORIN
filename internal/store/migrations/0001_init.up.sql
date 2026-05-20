-- Initial schema for orin MVP.
-- All identifiers are UUIDs generated app-side (we don't rely on a pg extension).

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    email       TEXT NOT NULL UNIQUE,
    role        TEXT NOT NULL DEFAULT 'admin',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE clusters (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL UNIQUE,
    server_url            TEXT NOT NULL,
    ca_cert               BYTEA,
    auth_config_encrypted BYTEA,
    in_cluster            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE repositories (
    id                    TEXT PRIMARY KEY,
    url                   TEXT NOT NULL UNIQUE,
    type                  TEXT NOT NULL DEFAULT 'git',
    credentials_encrypted BYTEA,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE applications (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL UNIQUE,
    project          TEXT NOT NULL DEFAULT 'default',
    repo_id          TEXT NOT NULL REFERENCES repositories(id) ON DELETE RESTRICT,
    path             TEXT NOT NULL,
    target_revision  TEXT NOT NULL DEFAULT 'HEAD',
    dest_cluster_id  TEXT NOT NULL REFERENCES clusters(id) ON DELETE RESTRICT,
    dest_namespace   TEXT NOT NULL,
    sync_policy_json JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX applications_repo_id_idx ON applications(repo_id);
CREATE INDEX applications_dest_cluster_id_idx ON applications(dest_cluster_id);

CREATE TABLE application_status (
    app_id            TEXT PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    sync_status       TEXT NOT NULL DEFAULT 'Unknown',
    health_status     TEXT NOT NULL DEFAULT 'Unknown',
    observed_revision TEXT NOT NULL DEFAULT '',
    last_synced_at    TIMESTAMPTZ,
    message           TEXT NOT NULL DEFAULT '',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sync_operations (
    id              TEXT PRIMARY KEY,
    app_id          TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    revision        TEXT NOT NULL DEFAULT '',
    initiated_by    TEXT NOT NULL DEFAULT 'system',
    status          TEXT NOT NULL DEFAULT 'Pending',
    message         TEXT NOT NULL DEFAULT '',
    resources_json  JSONB NOT NULL DEFAULT '[]'::JSONB
);

CREATE INDEX sync_operations_app_id_started_idx
    ON sync_operations(app_id, started_at DESC);

CREATE TABLE audit_log (
    id           TEXT PRIMARY KEY,
    ts           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor        TEXT NOT NULL,
    action       TEXT NOT NULL,
    resource     TEXT NOT NULL,
    payload_json JSONB
);

CREATE INDEX audit_log_ts_idx ON audit_log(ts DESC);
