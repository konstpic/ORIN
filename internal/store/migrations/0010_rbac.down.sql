-- Drop RBAC tables
DROP TABLE IF EXISTS role_bindings;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;

-- Remove added columns from users
ALTER TABLE users DROP COLUMN IF EXISTS token_hash;
ALTER TABLE users DROP COLUMN IF EXISTS active;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
