# ORIN Helm chart (scaled)

Umbrella chart: **apiserver**, **controller**, **reposerver**, **postgres**, **common** (subcharts under `charts/`).

| Subchart | Role |
|----------|------|
| `common` | ServiceAccount, Secret, RBAC |
| `postgres` | PostgreSQL (optional) |
| `reposerver` | gRPC renderer + HPA |
| `controller` | Reconciliation + optional apps catalog |
| `apiserver` | HTTP API + UI |

CRDs (`Application`, `AppProject`) ship in `crds/` and install with the chart.

## Images (GHCR)

Published on every `v*` tag and `main` push ([`.github/workflows/ghcr.yaml`](../../.github/workflows/ghcr.yaml)):

| Component | Image |
|-----------|--------|
| apiserver | `ghcr.io/konstpic/orin-apiserver` |
| controller | `ghcr.io/konstpic/orin-controller` |
| reposerver | `ghcr.io/konstpic/orin-reposerver` |
| all-in-one (dev) | `ghcr.io/konstpic/orin` |

Tag `v0.2.1` → image tags `0.2.1` (+ `latest` on `main`).

## Install from Git

```bash
helm upgrade --install orin ./deploy/helm \
  --namespace orin --create-namespace \
  --set global.image.tag=0.2.1
```

## Install from OCI (release tag)

```bash
helm registry login ghcr.io -u YOUR_GITHUB_USER
helm upgrade --install orin oci://ghcr.io/konstpic/charts/orin \
  --version 0.2.1 \
  --namespace orin --create-namespace
```

## Package locally

```bash
make helm-package
# charts/orin-0.2.1.tgz + charts/index.yaml
```

## Install from Git `charts/` repo

```bash
helm repo add orin https://konstpic.github.io/ORIN/charts/  # or raw Git path
helm upgrade --install orin orin/orin --version 0.2.1 -n orin --create-namespace
```

Docker Desktop: [`../docker-desktop/deploy.sh`](../docker-desktop/deploy.sh).
