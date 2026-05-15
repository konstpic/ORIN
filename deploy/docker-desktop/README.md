# Docker Desktop Kubernetes

Helm chart + script: build the app image, install **Postgres + k8s-ui** in namespace `k8s-ui`.

## How the image gets into the cluster

Many Docker Desktop setups **do not** share the host `docker build` image store
with Kubernetes containerd. The old `imagePullPolicy: Never` + `k8s-ui:local`
pattern then fails with **ErrImageNeverPull**.

**Default (recommended):** `./deploy/docker-desktop/deploy.sh` **pushes the
image to [ttl.sh](https://ttl.sh)** (anonymous HTTPS, no Docker Desktop extra
settings). Helm uses **`imagePullPolicy: Always`**. The URL is random and
time-limited (default **8h** TTL); treat it as **dev-only / effectively public**
to anyone who knows the name.

Override TTL tag (must be a ttl.sh duration, e.g. `24h`, `1h`):

```bash
TTL_SH_TTL=24h ./deploy/docker-desktop/deploy.sh
```

**Optional (advanced):** if your kubelet can already see `k8s-ui:local` on the
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
kubectl port-forward -n k8s-ui svc/k8s-ui 8080:80
```

Open **http://127.0.0.1:8080/** and sign in with **`devtoken`**.

## Tear down

```bash
helm uninstall k8s-ui -n k8s-ui
kubectl delete namespace k8s-ui
kubectl delete clusterrolebinding k8s-ui 2>/dev/null || true
```

## Files

- `values.yaml` — shared overrides (no image pull policy here; set by script).
- `values.never-pull.yaml` — local `k8s-ui:local` + `Never` (optional).
- `deploy.sh` — build, optional ttl.sh push, `helm upgrade --install`.
