ALTER TABLE applications
    DROP COLUMN IF EXISTS plugin_env_json,
    DROP COLUMN IF EXISTS plugin_name;

DROP TABLE IF EXISTS plugins;
