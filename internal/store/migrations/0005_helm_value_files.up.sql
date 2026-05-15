-- Ordered list of paths relative to the chart directory passed as -f layers
-- to helm template (Argo CD: spec.source.helm.valueFiles).
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS helm_value_files_json JSONB;
