-- Persist sync options (dry-run, prune override, partial resource selection) for the controller.
ALTER TABLE sync_operations
  ADD COLUMN IF NOT EXISTS request_json JSONB NOT NULL DEFAULT '{}'::jsonb;
