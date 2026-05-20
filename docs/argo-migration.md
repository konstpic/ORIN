# Migrating from Argo CD to orin

This guide covers what works as-is, what needs minor adaptation, and what is
not yet supported when moving an existing Argo CD installation to orin.

## Quick-start migration workflow

```
1. kubectl get applications.argoproj.io -A -o yaml > argo-apps.yaml
2. Register every Git repository that appears in your Application manifests
   via POST /api/v1/repositories (or the orin UI).
3. Register every target cluster via POST /api/v1/clusters.
4. POST /api/v1/argo-import  (dry-run first; see section below)
5. Disable Argo CD auto-sync on all migrated apps, verify orin shows Synced.
6. Remove Argo CD Applications.
```

---

## Feature compatibility matrix

### Application source

| Argo CD feature | orin status | Notes / migration path |
|-----------------|--------------|------------------------|
| `spec.source.repoURL` + `path` + `targetRevision` | **Supported** | Direct mapping. |
| `spec.source.helm.values` (inline YAML string) | **Supported** | Parsed and stored as `source.helmValues` JSON. |
| `spec.source.helm.valuesObject` (inline object) | **Supported** | Same storage path. |
| `spec.source.helm.valueFiles` (list of paths) | **Supported** | Resolved relative to the chart directory in the Git checkout. |
| `spec.source.helm.releaseName` | *Auto-derived* | Release name is sanitized from the Application name. Explicit override not yet exposed in the API. |
| `spec.source.helm.version` | Not supported | Pin the chart directory to the desired version in Git. |
| `spec.source.kustomize` (images, patches, namePrefix…) | Not supported | Move the overlay into the repo as a `kustomization.yaml` and point `path` at it. |
| `spec.source.plugin` / Config Management Plugin | Not supported | No planned support in v1. |
| `spec.sources` (multi-source) | **Partial** | At import time only the **first** source is used; a warning is logged. Multiple sources require creating one orin Application per source. |
| Helm chart from Helm repository / OCI registry | Not supported | The source must be in a registered Git repository. Planned for a future release. |
| Directory with `recurse: true` | *Default behavior* | Plain YAML renderer already recurses all `.yaml`/`.yml` files. |

### Sync policy

| Argo CD feature | orin status | Notes |
|-----------------|--------------|-------|
| `syncPolicy.automated.prune` | **Supported** | |
| `syncPolicy.automated.selfHeal` | **Supported** | |
| `syncPolicy.syncOptions: [CreateNamespace=true]` | **Supported** | Also settable via `syncPolicy.createNamespace: true`. |
| `syncPolicy.managedNamespaceMetadata` | **Supported** | `labels` and `annotations` merged into the Namespace object. |
| `syncPolicy.retry` | Not supported | Failed syncs must be retried manually or via automated selfHeal re-queue. |
| `argocd.argoproj.io/sync-wave` annotation | **Supported** | Resources are ordered by wave before apply. orin annotation `gitops.orin.dev/sync-wave` is equivalent. |
| `argocd.argoproj.io/hook` (PreSync / Sync / PostSync) | **Supported** | Hook phases are honoured. `SyncFail` maps to the "fail" bucket (applied last). orin annotation `gitops.orin.dev/hook` is equivalent. |
| Hook deletion policies | Not supported | Hooks are never deleted after sync. |
| `ApplyOutOfSyncOnly` sync option | Not supported | All desired resources are always applied. |
| `spec.ignoreDifferences` | **Supported** | Per-application rules; group/kind/name/namespace + JSON pointer paths. See below. |
| Server-side apply | **Default** | orin always uses SSA with `fieldManager: orin`. |
| Replace strategy | Not supported | |

### Projects and RBAC

| Argo CD feature | orin status | Notes |
|-----------------|--------------|-------|
| `AppProject` with `sourceRepos` whitelist | **Supported** | `project.spec.sourceRepos` is checked on create/update/sync. |
| `AppProject` with `destinations` whitelist | **Supported** | Cluster name and namespace patterns are validated. |
| `AppProject` with `clusterResourceWhitelist` | **Supported** | Cluster-scoped kinds must be in the project allow list. |
| `AppProject` with `namespaceResourceBlacklist` | **Supported** | Namespace-scoped kinds blocked for the project. |
| `AppProject` RBAC roles (`policies`) | Not supported | orin uses a single admin token today. OIDC + project roles are planned. |
| Orphaned resources monitoring | Not supported | |

### ApplicationSet

| Argo CD feature | orin status | Notes |
|-----------------|--------------|-------|
| Static list generator | **Equivalent** — use the [Apps Catalog](../README.md#application-catalog) or `POST /api/v1/application-batches`. |
| Git file/directory generator | **Equivalent** — point `APPS_CATALOG_REPO_URL` at the same repo; the catalog file is your generator. |
| App-of-apps / template generator | **Supported** — render `orin.io/v1alpha1 Application` objects from a parent chart; orin automatically upserts them. See [App of apps](../README.md#app-of-apps-child-applications-and-projects). |
| Cluster generator | Not supported | Manually create one Application per cluster or script via the batch API. |
| Matrix / merge generators | Not supported | Compose via CI scripts that call the batch API. |
| Pull Request generator | Not supported | |
| Progressive sync (waves, rollout) | Not supported | |

### Deployment history and rollback

| Argo CD feature | orin status | Notes |
|-----------------|--------------|-------|
| Deployment history list | **Supported** — `GET /api/v1/applications/{name}/history` |
| Rollback to a previous revision | **Supported** — `POST /api/v1/applications/{name}/rollback` with `{"revision": "<sha>"}` |
| Argo "last sync result" with resources | **Supported** — included in the sync history row. |

### Health assessment

| Kind | orin status |
|------|--------------|
| Deployment, ReplicaSet, StatefulSet, DaemonSet, Pod, Job | **Built-in** |
| Service, Ingress, ConfigMap, Secret, RBAC resources, Namespace | Always **Healthy** |
| Custom resources / other kinds | **Unknown** (Argo uses Lua scripts; planned for a future release) |

---

## Importing Argo CD Applications

orin exposes `POST /api/v1/argo-import` which accepts a raw Kubernetes YAML
document (one or more `argoproj.io/v1alpha1 Application` manifests, as produced
by `kubectl get applications.argoproj.io -A -o yaml`).

### Dry-run first

```bash
curl -s -X POST https://<orin>/api/v1/argo-import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @argo-apps.yaml \
  | jq .
```

Without the `?apply=true` query parameter the endpoint returns a preview of
what would be created/updated and lists unsupported features (multi-source,
Helm repo, plugins) as warnings. No changes are made.

### Apply

```bash
curl -s -X POST "https://<orin>/api/v1/argo-import?apply=true" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @argo-apps.yaml \
  | jq .
```

Each item in the response array contains:
- `name` — Application name
- `action` — `created`, `updated`, or `skipped`
- `warnings` — list of unsupported features that were silently dropped
- `error` — non-empty when the import failed for this item

### Prerequisites

Before running the import, register every repository and cluster that appears
in your Application manifests:

```bash
# Register a repository
curl -X POST https://<orin>/api/v1/repositories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://github.com/org/repo.git","username":"...","password":"..."}'

# Register an external cluster (kubeconfig YAML)
curl -X POST https://<orin>/api/v1/clusters \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"prod\",\"kubeconfigYaml\":\"$(cat ~/.kube/prod.yaml | base64)\"}"
```

The in-cluster API server (`https://kubernetes.default.svc`) is always
pre-registered as `in-cluster` and does not need to be created.

---

## `ignoreDifferences`

In Argo CD you can suppress false OutOfSync signals with `spec.ignoreDifferences`.
orin supports an equivalent in `syncPolicy.ignoreDifferences`:

```yaml
# orin catalog / API format
syncPolicy:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas          # ignore HPA-managed replica count
    - group: ""
      kind: ConfigMap
      name: my-config             # target a specific resource
      namespace: my-ns
      jsonPointers:
        - /data/generated-field
```

The same shape is accepted by `POST /api/v1/applications` and the catalog YAML.

Each rule matches resources by `group`, `kind`, and optionally `name` /
`namespace`. `jsonPointers` lists [RFC 6901](https://www.rfc-editor.org/rfc/rfc6901)
paths that are removed from **both** the desired and live objects before the
diff comparison is performed. A resource is considered **Synced** if the
remaining (non-ignored) fields are equal.

### Argo CD migration

```yaml
# Argo CD
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas

# orin (identical shape — paste as-is under syncPolicy)
syncPolicy:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas
```

---

## Migrating ApplicationSet

### Strategy decision

orin does **not** implement the full ApplicationSet controller. The decision
is intentional: ApplicationSet generators add significant complexity and most
real-world GitOps setups use only a narrow subset of their capabilities.

The table below maps each common generator to a orin equivalent:

| Argo CD generator | orin equivalent |
|-------------------|-------------------|
| **Static list** generator | `POST /api/v1/application-batches` with a JSON `items[]` list, or a Git catalog file (`applications:` list). |
| **Git file** generator (`path: apps/**/*.yaml`) | The [Apps Catalog](../README.md#application-catalog) — point `APPS_CATALOG_REPO_URL` at the same repo and commit a single `orin/apps.yaml` that lists all applications. |
| **Git directory** generator | Same as above: commit one entry per directory in the catalog file. |
| **Cluster** generator (one app per cluster) | Use `POST /api/v1/application-batches` from CI (e.g. a GitHub Actions workflow that iterates `kubectl get clusters` and calls the API). |
| **Matrix / merge** generator | Compose in CI scripts that call the batch API. |
| **Pull Request** generator | Create ephemeral apps from your CI pipeline on PR open and delete them on PR close via `DELETE /api/v1/applications/{name}`. |

### Git catalog — drop-in replacement for the list generator

An Argo `ApplicationSet` with a static list generator like:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  generators:
    - list:
        elements:
          - cluster: production
            url: https://prod.example.com
          - cluster: staging
            url: https://staging.example.com
  template:
    spec:
      source:
        repoURL: https://github.com/org/gitops.git
        targetRevision: HEAD
        path: environments/{{cluster}}
      destination:
        server: "{{url}}"
        namespace: myapp
```

translates to a Git catalog file (`orin/apps.yaml`):

```yaml
applications:
  - name: myapp-production
    source:
      repoUrl: https://github.com/org/gitops.git
      targetRevision: HEAD
      path: environments/production
    destination:
      cluster: production        # must match a registered orin cluster name
      namespace: myapp
  - name: myapp-staging
    source:
      repoUrl: https://github.com/org/gitops.git
      targetRevision: HEAD
      path: environments/staging
    destination:
      cluster: staging
      namespace: myapp
```

Enable catalog sync in `deploy/helm/values.yaml`:

```yaml
appsCatalog:
  enabled: true
  repoUrl: https://github.com/org/gitops.git
  path: orin/apps.yaml
  revision: HEAD
```

The controller polls this file every 5 minutes (configurable) and creates or
updates Application rows. Applications removed from the file are **not**
automatically deleted (to prevent accidental wipe).

### Batch API — scripted multi-cluster rollout

For dynamic scenarios (cluster generator, matrix) use CI to call the batch
endpoint. Example with `curl` and `jq`:

```bash
CLUSTERS=$(kubectl get clusters -o json | jq -r '.items[].metadata.name')
TEMPLATE='{"name":"APP_NAME","source":{"repoUrl":"...","path":"environments/APP_NAME","targetRevision":"HEAD"},"destination":{"cluster":"CLUSTER","namespace":"myapp"}}'
ITEMS=()
for cluster in $CLUSTERS; do
  ITEMS+=("{\"name\":\"myapp-${cluster}\",\"cluster\":\"${cluster}\"}")
done
curl -X POST https://<orin>/api/v1/application-batches \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"template\":${TEMPLATE},\"items\":[$(IFS=,; echo "${ITEMS[*]}")]}"
```

### Progressive delivery

ApplicationSet progressive sync (rollout waves across clusters) is not
supported. For canary/blue-green use a dedicated tool such as Argo Rollouts or
Flagger alongside orin; orin honours the `sync-wave` annotation to order
resources within a single sync.

---

## Known unsupported features

The following Argo CD features are **not** planned for v1 and have no migration
path beyond manual workarounds:

| Feature | Workaround |
|---------|-----------|
| Config Management Plugins (CMP) | Pre-render manifests in CI and commit them to Git as plain YAML. |
| Helm chart from Helm repo / OCI | Mirror the chart into a Git repository. |
| `ApplicationSet` cluster/PR/matrix generators | Use CI scripting + the batch API. |
| Argo CD RBAC policies (Casbin) | A single admin token is used today; OIDC + project roles are planned. |
| Notification controller | orin has a basic notification webhook; Argo-style triggers are not supported. |
| `argocd app wait` / gRPC streaming | Use the REST API + WebSocket `/api/v1/applications/{name}/events`. |
| Resource hooks with delete policies | Hooks fire but are never deleted post-sync. |
