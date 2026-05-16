-- RBAC: roles, role_permissions, and role_bindings tables.
-- The users table already exists (0001_init); we add rbac columns to it.

-- Roles table
CREATE TABLE roles (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    built_in     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Role permissions (many-to-many)
CREATE TABLE role_permissions (
    role_id     TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission  TEXT NOT NULL,
    PRIMARY KEY (role_id, permission)
);

-- Role bindings: map users to roles, optionally scoped to projects
CREATE TABLE role_bindings (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id    TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    projects   JSONB NOT NULL DEFAULT '[]'::JSONB, -- empty array = all projects
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, role_id, projects)
);

CREATE INDEX role_bindings_user_id_idx ON role_bindings(user_id);
CREATE INDEX role_bindings_role_id_idx ON role_bindings(role_id);

-- Add token_hash column to users for token-based auth (stores bcrypt hash)
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_hash TEXT;
-- Add active flag to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
-- Add display_name to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

-- Seed built-in roles
INSERT INTO roles (id, name, display_name, description, built_in) VALUES
    ('rbac-admin',     'admin',  'Administrator', 'Full access to all resources and RBAC management', true),
    ('rbac-editor',    'editor', 'Editor',        'Can create, update, and sync applications within assigned projects', true),
    ('rbac-viewer',    'viewer', 'Viewer',        'Read-only access within assigned projects', true);

-- Admin gets all permissions
INSERT INTO role_permissions (role_id, permission) VALUES
    ('rbac-admin', 'applications:list'),
    ('rbac-admin', 'applications:get'),
    ('rbac-admin', 'applications:create'),
    ('rbac-admin', 'applications:update'),
    ('rbac-admin', 'applications:delete'),
    ('rbac-admin', 'applications:sync'),
    ('rbac-admin', 'applications:rollback'),
    ('rbac-admin', 'applications:refresh'),
    ('rbac-admin', 'pods:logs'),
    ('rbac-admin', 'pods:exec'),
    ('rbac-admin', 'pods:shell'),
    ('rbac-admin', 'pods:delete'),
    ('rbac-admin', 'live-resource:get'),
    ('rbac-admin', 'live-resource:edit'),
    ('rbac-admin', 'live-resource:apply'),
    ('rbac-admin', 'live-resource:delete'),
    ('rbac-admin', 'live-resource:restart'),
    ('rbac-admin', 'repositories:list'),
    ('rbac-admin', 'repositories:get'),
    ('rbac-admin', 'repositories:create'),
    ('rbac-admin', 'repositories:delete'),
    ('rbac-admin', 'clusters:list'),
    ('rbac-admin', 'clusters:get'),
    ('rbac-admin', 'clusters:create'),
    ('rbac-admin', 'clusters:delete'),
    ('rbac-admin', 'projects:list'),
    ('rbac-admin', 'projects:get'),
    ('rbac-admin', 'projects:create'),
    ('rbac-admin', 'projects:update'),
    ('rbac-admin', 'projects:delete'),
    ('rbac-admin', 'notifications:list'),
    ('rbac-admin', 'notifications:create'),
    ('rbac-admin', 'notifications:update'),
    ('rbac-admin', 'notifications:delete'),
    ('rbac-admin', 'notifications:test'),
    ('rbac-admin', 'hooks:list'),
    ('rbac-admin', 'hooks:create'),
    ('rbac-admin', 'hooks:update'),
    ('rbac-admin', 'hooks:delete'),
    ('rbac-admin', 'rbac:manage'),
    ('rbac-admin', 'audit:view');

-- Editor permissions
INSERT INTO role_permissions (role_id, permission) VALUES
    ('rbac-editor', 'applications:list'),
    ('rbac-editor', 'applications:get'),
    ('rbac-editor', 'applications:create'),
    ('rbac-editor', 'applications:update'),
    ('rbac-editor', 'applications:sync'),
    ('rbac-editor', 'applications:rollback'),
    ('rbac-editor', 'applications:refresh'),
    ('rbac-editor', 'pods:logs'),
    ('rbac-editor', 'pods:exec'),
    ('rbac-editor', 'pods:shell'),
    ('rbac-editor', 'live-resource:get'),
    ('rbac-editor', 'live-resource:edit'),
    ('rbac-editor', 'live-resource:apply'),
    ('rbac-editor', 'live-resource:restart'),
    ('rbac-editor', 'repositories:list'),
    ('rbac-editor', 'repositories:get'),
    ('rbac-editor', 'clusters:list'),
    ('rbac-editor', 'clusters:get'),
    ('rbac-editor', 'projects:list'),
    ('rbac-editor', 'projects:get'),
    ('rbac-editor', 'notifications:list'),
    ('rbac-editor', 'notifications:create'),
    ('rbac-editor', 'notifications:update'),
    ('rbac-editor', 'notifications:delete'),
    ('rbac-editor', 'hooks:list'),
    ('rbac-editor', 'hooks:create'),
    ('rbac-editor', 'hooks:update'),
    ('rbac-editor', 'hooks:delete');

-- Viewer permissions
INSERT INTO role_permissions (role_id, permission) VALUES
    ('rbac-viewer', 'applications:list'),
    ('rbac-viewer', 'applications:get'),
    ('rbac-viewer', 'applications:refresh'),
    ('rbac-viewer', 'pods:logs'),
    ('rbac-viewer', 'live-resource:get'),
    ('rbac-viewer', 'repositories:list'),
    ('rbac-viewer', 'repositories:get'),
    ('rbac-viewer', 'clusters:list'),
    ('rbac-viewer', 'clusters:get'),
    ('rbac-viewer', 'projects:list'),
    ('rbac-viewer', 'projects:get'),
    ('rbac-viewer', 'notifications:list'),
    ('rbac-viewer', 'hooks:list');
