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

## Images (Docker Hub)

Public images on [Docker Hub — konstpic](https://hub.docker.com/u/konstpic):

| Component | Image |
|-----------|--------|
| apiserver | `konstpic/orin-apiserver` |
| controller | `konstpic/orin-controller` |
| reposerver | `konstpic/orin-reposerver` |

Default chart tag: **0.2.2** (`global.image.tag`). Override:

```bash
helm upgrade --install orin ./deploy/helm \
  --namespace orin --create-namespace \
  --set global.image.tag=0.2.2
```

## Install from Git

```bash
helm upgrade --install orin ./deploy/helm \
  --namespace orin --create-namespace
```

## Install from `charts/` Helm repo

```bash
helm repo add orin https://raw.githubusercontent.com/konstpic/ORIN/main/charts
helm search repo orin
helm upgrade --install orin orin/orin --version 0.2.3 -n orin --create-namespace
```

## Package locally

```bash
make helm-package
# charts/orin-<version>.tgz + charts/index.yaml
```

Docker Desktop: [`../docker-desktop/deploy.sh`](../docker-desktop/deploy.sh) (`USE_DOCKERHUB=1` to rebuild and push).
