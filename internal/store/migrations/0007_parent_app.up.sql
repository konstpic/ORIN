ALTER TABLE applications ADD COLUMN IF NOT EXISTS parent_app TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_applications_parent_app ON applications (parent_app) WHERE parent_app <> '';
