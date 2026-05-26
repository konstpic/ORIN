# Docker Desktop Kubernetes

Helm umbrella chart + script: build the app image, install **Postgres + apiserver +
controller + reposerver** in namespace `orin` (separate subcharts, horizontally scalable).

## How the image gets into the cluster

Many Docker Desktop setups **do not** share the host `docker build` image store
with Kubernetes containerd. The old `imagePullPolicy: Never` + `orin:local`
pattern then fails with **ErrImageNeverPull**.

**Default (recommended):** `./deploy/docker-desktop/deploy.sh` builds **three
component images** (`orin-apiserver`, `orin-controller`, `orin-reposerver`) and
**pushes each to [ttl.sh](https://ttl.sh)** (anonymous HTTPS). Helm uses
**`imagePullPolicy: Always`**. URLs are random and time-limited (default **8h**
TTL); treat them as **dev-only / effectively public** to anyone who knows the name.

Override TTL tag (must be a ttl.sh duration, e.g. `24h`, `1h`):

```bash
TTL_SH_TTL=24h ./deploy/docker-desktop/deploy.sh
```

**Docker Hub (recommended when GitHub Actions is unavailable):** build, push,
and deploy from your Hub account (log in via Docker Desktop first):

```bash
USE_DOCKERHUB=1 DOCKERHUB_USER=konstpic DOCKERHUB_TAG=0.2.2 ./deploy/docker-desktop/deploy.sh
```

Images: `konstpic/orin-apiserver`, `orin-controller`, `orin-reposerver` (public
repos pull without a secret; private repos need `DOCKERHUB_TOKEN`).

**Optional (advanced):** if your kubelet can already see `orin:local` on the
node where the pod runs:

```bash
USE_LOCAL_IMAGE_NEVER=1 ./deploy/docker-desktop/deploy.sh
```

That uses `values.never-pull.yaml` (`Never` + local tag). On stock Docker
Desktop this usually **still fails** unless you use another mechanism to load
the image into the node’s store.

## Prerequisites

1. **Docker Desktop** → Kubernetes enabled and running.
2. `kubectl config use-context docker-desktop`
3. **Helm 3** (`helm version`)
4. Outbound **HTTPS** to `ttl.sh` (default deploy path).

## Multi-node

With the default **ttl.sh** flow, multi-node is fine. The old **Never** + local
image guard (single-node) applies only when `USE_LOCAL_IMAGE_NEVER=1`.

## Deploy

```bash
chmod +x deploy/docker-desktop/deploy.sh
./deploy/docker-desktop/deploy.sh
```

Port-forward (Docker Desktop often does not map `NodePort` to localhost well):

```bash
kubectl port-forward -n orin svc/orin-apiserver 8080:80
```

Open **http://127.0.0.1:8080/** and sign in with **`devtoken`**.

## Tear down

```bash
helm uninstall orin -n orin
kubectl delete namespace orin
kubectl delete clusterrolebinding orin-orin 2>/dev/null || true
```

## Files

- `values.yaml` — shared overrides (no image pull policy here; set by script).
- `values.never-pull.yaml` — local `orin:local` + `Never` (optional).
- `deploy.sh` — build, optional ttl.sh push, `helm upgrade --install` (scaled subcharts).
- `deploy-scaled.sh` — alias for `deploy.sh` (same Helm chart).
