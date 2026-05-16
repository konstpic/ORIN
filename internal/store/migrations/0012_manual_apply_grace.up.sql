-- Add last_manual_apply_at to application_status to persist manual apply
-- timestamps across pod restarts and leader failovers. This is used to
-- suppress auto-sync during the grace period after a manual live-apply.
ALTER TABLE application_status
  ADD COLUMN IF NOT EXISTS last_manual_apply_at TIMESTAMPTZ;
