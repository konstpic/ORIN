-- AppProject-style policy constraints stored as JSON arrays.
-- sourceRepos:                 list of allowed repoURL patterns ("*" = any)
-- destinations:                list of {server|name, namespace} patterns
-- clusterResourceWhitelist:    list of {group, kind} cluster-scoped resources allowed
-- namespaceResourceBlacklist:  list of {group, kind} namespace-scoped resources denied
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS policies_json JSONB NOT NULL DEFAULT '{}';
