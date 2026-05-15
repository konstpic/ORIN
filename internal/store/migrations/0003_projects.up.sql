-- Argo-style projects table (RBAC policies attach here in a future migration).

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO projects (id, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Built-in project')
ON CONFLICT (name) DO NOTHING;
