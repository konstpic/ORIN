#!/usr/bin/env bash
# Deploy k8s-ui in scaled mode (separate apiserver, controller, reposerver)
# to Docker Desktop Kubernetes (context docker-desktop).
#
# Prerequisites: Docker Desktop with Kubernetes enabled, kubectl.
#
# This script deploys 3 independent components:
#   apiserver   — stateless HTTP + WebSocket gateway
#   controller  — reconciliation loop with leader election
#   reposerver  — gRPC manifest renderer (HPA)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS=k8s-ui
THIS_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Checking kubectl context (expect docker-desktop)"
CTX="$(kubectl config current-context 2>/dev/null || true)"
if [[ "${CTX}" != "docker-desktop" ]] && [[ "${SKIP_CONTEXT_CHECK:-}" != "1" ]]; then
  echo "Error: current kubectl context is '${CTX}', expected 'docker-desktop'."
  echo "Run: kubectl config use-context docker-desktop"
  echo "Or override: SKIP_CONTEXT_CHECK=1 $0"
  exit 1
fi

echo "==> Building image k8s-ui:local"
docker build --no-cache -t k8s-ui:local "${ROOT}"

NODES="$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"
USE_NEVER="${USE_LOCAL_IMAGE_NEVER:-0}"
if [[ "${NODES}" != "1" ]] && [[ "${ALLOW_MULTI_NODE:-}" != "1" ]] && [[ "${USE_NEVER}" == "1" ]]; then
  echo "Warning: this cluster has ${NODES} nodes. With imagePullPolicy:Never the image must exist on the target node."
  echo "Re-run with: ALLOW_MULTI_NODE=1 $0"
  exit 1
fi

# Image handling: ttl.sh or local
IMAGE="k8s-ui:local"
PULL_POLICY="Never"

if [[ "${USE_NEVER}" != "1" ]]; then
  TTL_SUFFIX="${TTL_SH_TTL:-8h}"
  RAND="$(openssl rand -hex 10)"
  TTL_IMAGE="ttl.sh/k8s-ui-${RAND}:${TTL_SUFFIX}"
  echo "==> Pushing to ${TTL_IMAGE} (ephemeral; dev-only)"
  docker tag k8s-ui:local "${TTL_IMAGE}"
  docker push "${TTL_IMAGE}"
  IMAGE="${TTL_IMAGE}"
  PULL_POLICY="Always"
fi

echo "==> Creating namespace ${NS}"
kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f -

# --- Postgres (StatefulSet from scaled manifests) ---
echo "==> Deploying Postgres"

# --- Postgres (StatefulSet) ---
# Reuse existing Postgres if present (from all-in-one deployment).
# Do NOT delete or recreate the Service — data lives in PVC, not in the Service,
# but changing clusterIP causes errors on existing Services.
echo "==> Checking Postgres"
if kubectl get statefulset k8s-ui-postgres -n "${NS}" &>/dev/null; then
  echo "  Postgres already exists, reusing it"
else
  echo "  Deploying new Postgres"
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: k8s-ui-postgres
  namespace: ${NS}
spec:
  selector:
    app: k8s-ui-postgres
  ports:
    - port: 5432
      targetPort: 5432
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: k8s-ui-postgres
  namespace: ${NS}
spec:
  serviceName: k8s-ui-postgres
  selector:
    matchLabels:
      app: k8s-ui-postgres
  template:
    metadata:
      labels:
        app: k8s-ui-postgres
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports: [{ containerPort: 5432 }]
          env:
            - name: POSTGRES_USER
              value: k8sui
            - name: POSTGRES_PASSWORD
              value: k8sui
            - name: POSTGRES_DB
              value: k8sui
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "k8sui"]
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
EOF
fi

echo "==> Waiting for Postgres"
kubectl rollout status statefulset/k8s-ui-postgres -n "${NS}" --timeout=120s

# --- Secret ---
echo "==> Creating secrets"
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: k8s-ui-secret
  namespace: ${NS}
type: Opaque
stringData:
  database-url: "postgres://k8sui:k8sui@k8s-ui-postgres:5432/k8sui?sslmode=disable"
  admin-token: "devtoken"
  encryption-key: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
EOF

# --- RBAC (ClusterRole for in-cluster access) ---
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8s-ui
  namespace: ${NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-ui-cluster-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: k8s-ui
    namespace: ${NS}
EOF

# --- Deploy scaled components ---
IMAGE_ENV=$(echo "${IMAGE}" | cut -d: -f1)
IMAGE_TAG=$(echo "${IMAGE}" | cut -d: -f2 || echo "local")

echo "==> Deploying RepoServer (gRPC, image=${IMAGE})"
sed "s|image: k8s-ui:dev|image: ${IMAGE}|g; s|args: \[\"reposerver\"\]|args: [\"reposerver\"]|g" \
  "${ROOT}/deploy/scaled/03-reposerver.yaml" | \
  kubectl apply -f -

echo "==> Deploying Controller (leader election)"
sed "s|image: k8s-ui:dev|image: ${IMAGE}|g" \
  "${ROOT}/deploy/scaled/02-controller.yaml" | \
  kubectl apply -f -

echo "==> Deploying API Server (stateless)"
sed "s|image: k8s-ui:dev|image: ${IMAGE}|g" \
  "${ROOT}/deploy/scaled/01-apiserver.yaml" | \
  kubectl apply -f -

echo "==> Waiting for workloads"
kubectl rollout status deployment/k8s-ui-reposerver -n "${NS}" --timeout=120s || true
kubectl rollout status deployment/k8s-ui-controller -n "${NS}" --timeout=120s || true
kubectl rollout status deployment/k8s-ui-apiserver -n "${NS}" --timeout=120s

PF_PORT="${PF_PORT:-8080}"
echo ""
echo "=== Scaled deployment finished ==="
echo ""
echo "Components:"
kubectl get pods -n "${NS}" -l app.kubernetes.io/name=k8s-ui-apiserver -o wide 2>/dev/null || true
kubectl get pods -n "${NS}" -l app.kubernetes.io/name=k8s-ui-controller -o wide 2>/dev/null || true
kubectl get pods -n "${NS}" -l app.kubernetes.io/name=k8s-ui-reposerver -o wide 2>/dev/null || true
echo ""
echo "Access:"
echo "  kubectl port-forward -n ${NS} svc/k8s-ui-apiserver ${PF_PORT}:80"
echo "  http://127.0.0.1:${PF_PORT}/"
echo ""
echo "Sign in with token: devtoken"
echo ""
if [[ "${USE_NEVER}" != "1" ]]; then
  echo "Note: app image was pushed to ttl.sh (public URL, TTL ${TTL_SUFFIX:-8h})."
fi
