# Roadmap: parity with Argo CD

This document fixes the **target horizon** for k8s_ui relative to [Argo CD](https://argo-cd.readthedocs.io/) and maps work to phases. It complements [design.md](design.md).

## Target horizons

1. **Single-cluster product (default near-term goal)**  
   One Kubernetes API target, strong GitOps UX (render, diff, sync, history, pod tooling), optional Helm/Kustomize, revision compare/rollback, basic metrics. **No full OIDC/RBAC matrix required** to ship value.

2. **Multi-cluster + enterprise auth**  
   Per-cluster kubeconfigs, project-scoped RBAC, OIDC/SSO, audit exports, notifications — required for orgs that treat Argo CD as a control plane.

3. **Full Argo-style platform**  
   ApplicationSet-scale generation, sync hooks/waves parity with Argo resource hooks, CMP/plugins, custom Lua health, deep integrations (Slack, badges, extensions API).

**Current repo stance:** implement **(1)** first in the open-source tree; design and stub **(2)** where low-cost; document **(3)** as stretch.

## Phase map

| Phase | Theme | Status in repo |
|-------|--------|----------------|
| P1 | Helm/Kustomize render, revision list/diff/rollback | Implemented incrementally |
| P1b | Prometheus `/metrics`, optional webhook | Implemented incrementally |
| P2 | Projects table + list API; RBAC/OIDC hooks | Projects + RBAC stub; OIDC config placeholders |
| P2b | Cluster registration API (encrypted kubeconfig persisted) | Implemented |
| P3 | Sync wave ordering (Argo-compatible annotation) | Implemented |
| P3b | Application batch create (minimal “AppSet-lite”) | Implemented |
| P4 | Multi-cluster execution (controller selects client per app) | Implemented: `kubeClientForApp` + `RemoteCluster` for non–in-cluster rows |

## Sync policy notes (P3 extensions)

- **Hooks:** `argocd.argoproj.io/hook` / `gitops.k8s-ui.dev/hook` (`PreSync`, `PostSync`, …) order resources before sync-wave ordering within each phase.
- **Sync windows:** `SYNC_DENY_RANGE_UTC` (e.g. `22:00-06:00`) blocks manual API sync and auto-sync during the UTC window.
- **Retries:** `SYNC_APPLY_RETRIES` (default 1) retries each resource apply with linear backoff.

## How we use the local Argo CD checkout

Use `/path/to/argo-cd` only as a **reference** for API shapes and UX patterns (e.g. `server/application`, `reposerver/repository`). k8s_ui stays a smaller REST surface and does not embed Argo’s gRPC stack.
