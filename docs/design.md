# Design

This document is a copy of the technical design that the project was built
to. The authoritative version of the plan lives in `.cursor/plans/`.

See [README.md](../README.md) for repository layout and quick start.

## Summary

- **Backend:** Go single-binary with three logical roles (`apiserver`,
  `controller`, `reposerver`). MVP runs them in-process via the
  `all-in-one` subcommand; future deployments may split them.
- **Frontend:** Vite + React 18 + TypeScript + Tailwind. Communicates over
  REST and a multiplexed WebSocket.
- **Persistence:** PostgreSQL, accessed via `pgx` + hand-written SQL.
  Schema migrations via golang-migrate embedded in the binary.
- **K8s integration:** client-go dynamic informers; server-side apply with
  field manager `orin`; tracking label
  `app.kubernetes.io/instance=<appName>` ties live resources back to apps.

## Components

```
+----------+    REST/WS    +-----------+   gRPC*   +-----------+
| Browser  | <-----------> | API Server| <-------> | RepoServer|
+----------+               +-----------+           +-----------+
                                 |                       |
                          informer cache             git clone+render
                                 v                       v
                          +-----------+            +-----------+
                          | Cluster   |            | Disk cache|
                          +-----------+            +-----------+
                                 ^
                                 |
                          +-----------+
                          |Controller |  workqueue + reconcile
                          +-----------+
                                 |
                                 v
                          +-----------+
                          | Postgres  |
                          +-----------+
```
*MVP: in-process function calls.*

## Data model

| Table                | Purpose                                                         |
|----------------------|-----------------------------------------------------------------|
| `users`              | Stub for future RBAC                                            |
| `clusters`           | Managed K8s targets (MVP: single `in-cluster` row)              |
| `repositories`       | Git sources with encrypted credentials                          |
| `applications`       | Desired-state declarations                                      |
| `application_status` | Hot status row (1:1 with applications)                          |
| `sync_operations`    | History of apply attempts                                       |
| `audit_log`          | Append-only audit trail                                         |

## Reconcile loop

1. Status workqueue receives an app key (timed resync, Git poll, user refresh).
2. Repo server renders manifests at the resolved revision (with tracking
   labels injected).
3. Live state is read from informer caches (lazy-registered per GVR).
4. The diff engine normalizes both sides (strips status/managedFields/RV) and
   compares with `canonicalize` recursive structural equality.
5. `application_status` is upserted; a WebSocket frame is broadcast to
   `app:<name>:status` topic.
6. If `syncPolicy.automated` is set and the app is `OutOfSync`, a
   `SyncOperation` is enqueued automatically.

## Sync executor

1. Pulls oldest `Pending` sync op for the app.
2. Re-renders manifests at the requested revision.
3. Server-side applies each object with `fieldManager: orin`,
   `force: true`.
4. If `prune` is enabled, deletes any objects bearing the tracking label
   that are not present in the desired set.
5. Persists per-resource results in `sync_operations.resources_json`.

## Frontend

Single SPA; route map mirrors the design doc. WebSocket connection per app
detail page invalidates the relevant TanStack Query keys on push, so the UI
re-fetches automatically without manual polling loops.

## Future work

- Helm + Kustomize renderers (slot into `internal/manifest`).
- Webhook-based change detection (`POST /api/v1/webhook/{provider}`).
- OIDC + project-based RBAC.
- HA controller with leader election.
- Multi-cluster: persist kubeconfig per cluster row; `ClusterManager` spins
  up factories on demand.
- Prometheus metrics + OpenTelemetry tracing.
