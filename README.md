# ORIN — GitOps Navigation System for Kubernetes

`ORIN` is an ArgoCD-inspired GitOps platform written in Go (backend) and
React + TypeScript (frontend). Each *Application* declares a Git source (repo +
path + revision) and a destination (cluster + namespace). The system
continuously reconciles desired state (Git) with actual state (Kubernetes) and
exposes both via a dashboard.

> This repository is the MVP described in
> [`docs/design.md`](docs/design.md). It ships as a single Go binary with
> three subcommands (`apiserver`, `controller`, `reposerver`) plus a Vite
> React frontend.

## Quick start

```bash
# Backend
make build         # produce ./bin/orin
make test          # run unit tests

# Database (Postgres 15+)
createdb orin
DATABASE_URL=postgres://localhost/orin?sslmode=disable \
    ./bin/orin migrate up

# Run all three subprocesses in one binary
DATABASE_URL=postgres://localhost/orin?sslmode=disable \
ADMIN_TOKEN=secret \
ENCRYPTION_KEY=$(openssl rand -hex 32) \
    ./bin/orin all-in-one

# Frontend (dev)
cd web && npm install && npm run dev
```

By default the API server listens on `:8080` and the frontend dev server on
`:5173` (proxying `/api/*` to the backend).

## Helm charts from Git

If `source.path` points at a chart root (contains `Chart.yaml` or `Chart.yml`),
manifests are rendered with **Helm 3** (`helm template`). The chart’s own
`values.yaml` is used by default. Optional **`source.helmValues`** on the
Application (JSON object) is passed as an extra `-f` values layer (same as a
small `values.override.yaml`).

**One Application = one Git path.** `helm template` on an umbrella chart
(for example `deploy/.helm`) renders **every enabled subchart** into that
single sync: you will not get a second row in the UI unless you create another
Application. To deploy `samples/hello-world` on its own, add a **new**
Application (different name), set **path** to `samples/hello-world` (or your
leaf chart directory), and pick the destination namespace you want for that
chart. If the umbrella should no longer ship hello-world from the `deploy` app,
disable that dependency in the umbrella `values.yaml` or override it with
`source.helmValues` on the `deploy` application.

**Examples:** a repo may expose plain manifests under `kubernetes/` and a leaf
chart under `samples/hello-world/` — register two applications with paths
`kubernetes` and `samples/hello-world` respectively. An Argo-style *umbrella*
that only emits `Application` / `AppProject` CRDs (some layouts under
`deploy/.helm`) is meant for Argo CD; if your umbrella instead renders real
workloads, orin will apply all of them from that one Application.

The release image installs the `helm` binary; for local `all-in-one`, install
Helm v3 on your PATH.

**Troubleshooting plain YAML repos:** manifests are applied in hook/wave order,
then by resource kind so **`Namespace` runs before `Deployment`** even when Git
lists `deployment.yaml` before `namespace.yaml`.

## Application catalog (Git-driven app list)

To declare **several** ORIN applications (each with its own Git `repoUrl` and
`path`) from a single repository, enable the optional **apps catalog**:

1. Register every `source.repoUrl` you reference in the catalog (same as for
   manually created applications).
2. Commit a YAML file in one of those repos, for example `orin/apps.yaml`:

```yaml
# Canonical format: one document per object, separated by ---
apiVersion: orin.dev/v1alpha1
kind: Application
metadata:
  name: web
spec:
  project: default
  source:
    repoURL: https://github.com/org/gitops.git
    path: kubernetes
    targetRevision: main
  destination:
    name: in-cluster
    namespace: gitops-demo
---
apiVersion: orin.dev/v1alpha1
kind: Application
metadata:
  name: hello
spec:
  source:
    repoURL: https://github.com/org/gitops.git
    path: samples/hello-world
    targetRevision: main
  destination:
    name: in-cluster
    namespace: demo
  syncPolicy:
    automated:
      prune: false
      selfHeal: false
```

> **Legacy format (deprecated):** A single-document file with a top-level `applications:` list is still parsed but logs a deprecation warning on every poll. Migrate to the CRD format above.
>
> ```yaml
> # Deprecated — migrate to orin.dev/v1alpha1 Application objects above
> applications:
>   - name: web
>     ...
> ```

3. Set environment variables on the **controller** process (same binary as
   `all-in-one`):

| Variable | Meaning |
|----------|---------|
| `APPS_CATALOG_REPO_URL` | HTTPS URL of the repo that contains the catalog file (must be registered). If unset, catalog sync is disabled. |
| `APPS_CATALOG_PATH` | Path inside the repo (default `orin/apps.yaml`). |
| `APPS_CATALOG_REVISION` | Git ref to resolve (default `HEAD`). |
| `APPS_CATALOG_INTERVAL` | Poll interval (default `5m`, minimum `10s` when catalog is enabled). |

**Helm:** in `deploy/helm/values.yaml` set `appsCatalog.enabled: true` and
`appsCatalog.repoUrl` to the **exact** same string as the repository URL in
ORIN (including or omitting `.git` — it must match `Repositories`). Upgrade
the release so the pod gets `APPS_CATALOG_*` env vars. If those variables are
unset, `orin/apps.yaml` in Git is never read and no rows are created.

On each tick the controller **creates** missing applications and **updates**
changed fields. Applications **removed** from the YAML file are **not**
deleted (avoid accidental wipe). Optional `helmValues` under `source` is a
YAML mapping stored as JSON for `helm template`, same as `source.helmValues` on
the API.

### App of apps (child applications and projects)

After each successful **status reconcile**, the controller automatically scans
every rendered manifest from the parent Application for `orin.dev/v1alpha1`
or `argoproj.io/v1alpha1` `Application` and `AppProject` objects and **upserts**
them into the database. No flag is needed — this behaviour is always active.

**Child application template (in a parent chart's `templates/` dir):**

```yaml
apiVersion: orin.dev/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  project: default
  source:
    repoURL: https://github.com/org/child.git
    path: helm/child
    targetRevision: main
  destination:
    name: in-cluster   # registered ORIN cluster name
    namespace: child-ns
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    createNamespace: true
```

**Child project template:**

```yaml
apiVersion: orin.dev/v1alpha1
kind: AppProject
metadata:
  name: team-a
spec:
  description: "Team A workloads"
  sourceRepos:
    - "https://github.com/org/*"
  destinations:
    - name: in-cluster
      namespace: "team-a-*"
```

**Argo CD compat:** `argoproj.io/v1alpha1` `Application` and `AppProject`
manifests are recognised and produce the same result — useful for incremental
migration from Argo CD.

Control-plane objects (`orin.dev/*` and `argoproj.io` Application/AppProject)
are **never applied** to the destination cluster. Every `source.repoURL`
referenced by a child Application must be registered in ORIN. Projects are
upserted before Applications.

This is orthogonal to **`APPS_CATALOG_*`** (global file poll): app-of-apps
materialization is driven by whatever the **parent chart** renders each reconcile.
### Argo-style sync options (subset)

- **`syncPolicy.syncOptions`**: list of strings, Argo-compatible. **`CreateNamespace=true`**
  is honored the same way as **`syncPolicy.createNamespace`: `true`** (either
  or both may be set).
- **`syncPolicy.managedNamespaceMetadata`**: `labels` / `annotations` merged into
  the `Namespace` object when namespace creation runs.
- **`syncPolicy.createNamespace`**: shortcut bool (same effect as the sync option above).
- **`syncPolicy.automated`**: unchanged (prune / selfHeal). Not yet implemented from Argo:
  retry, resource hooks, server-side apply as first-class options, etc.

You can also drive creation from CI with **`POST /api/v1/application-batches`**:
the JSON `template` is merged into each `items[]` row; each item may override
`repoUrl`, `path`, `targetRevision`, `cluster`, `namespace` (`destNamespace`),
`project`, and `helmValues`.

## Docker Desktop Kubernetes

To run the whole stack (Postgres + API + embedded UI) inside Docker Desktop’s
Kubernetes, see [`deploy/docker-desktop/README.md`](deploy/docker-desktop/README.md)
and run `./deploy/docker-desktop/deploy.sh` (by default pushes a dev image to
[ttl.sh](https://ttl.sh) so the cluster can pull it without extra Docker Desktop settings).

## Repository layout

```
cmd/orin/           # Cobra CLI entry point with apiserver/controller/reposerver/all-in-one/migrate
internal/
  api/              # HTTP handlers, middleware, WebSocket hub
  auth/             # static-token middleware (OIDC pluggable later)
  config/           # env-driven config
  controller/       # workqueue, reconciler, sync executor
  crypto/           # AES-GCM secret encryption
  domain/           # Application / Cluster / Repository / SyncOperation types
  git/              # go-git wrapper
  k8s/              # ClusterManager, diff engine, apply
  appcatalog/       # YAML application list → domain (catalog + embedded children)
  reposerver/       # in-process repo+render service
  store/            # pgx-based Postgres repository layer
  ws/               # WebSocket hub (topic multiplexing)
pkg/api/v1/         # public DTOs / OpenAPI types
deploy/
  k8s/              # sample Kubernetes manifests
  helm/             # Helm chart (sketch)
migrations/         # golang-migrate SQL files
web/                # React + TS frontend
docs/design.md      # full technical design (mirrors plan)
test/e2e/           # kind-based end-to-end smoke test
```

## Migrating from Argo CD

See [`docs/argo-migration.md`](docs/argo-migration.md) for a full compatibility
matrix, a step-by-step import workflow (`POST /api/v1/argo-import`), and guides
for migrating `ignoreDifferences`, `AppProject` policies, and `ApplicationSet`.

## Documentation

* [`docs/design.md`](docs/design.md) – full architecture, GitOps flow, K8s
  interaction model.
* [`docs/argo-migration.md`](docs/argo-migration.md) – Argo CD compatibility matrix and migration guide.
* [`docs/api.md`](docs/api.md) – REST + WebSocket reference.
* [`docs/development.md`](docs/development.md) – local dev workflow.

## License

Apache 2.0
