-- Optional Helm values override (JSON object merged via helm template -f).
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS helm_values_json JSONB;
