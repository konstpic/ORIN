-- Plugins: globally-registered config management plugins (CMP-style).
CREATE TABLE plugins (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    -- generate_json stores PluginGenerateSpec as JSONB { command, args }.
    generate_json JSONB NOT NULL DEFAULT '{}'::JSONB,
    -- env_json stores []EnvVar as JSONB.
    env_json     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-application plugin reference.
ALTER TABLE applications
    ADD COLUMN plugin_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN plugin_env_json JSONB;
