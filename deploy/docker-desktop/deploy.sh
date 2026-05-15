#!/usr/bin/env bash
# Deploy k8s-ui to Docker Desktop Kubernetes (context docker-desktop).
# Prerequisites: Docker Desktop with Kubernetes enabled, helm, kubectl.
#
# Default: build k8s-ui:local, push to ttl.sh (ephemeral public URL), Helm uses
#   imagePullPolicy: Always — works when Docker and Kubernetes use separate image
#   stores (no "Use containerd for pulling…" option required).
#
# Optional: USE_LOCAL_IMAGE_NEVER=1 — use k8s-ui:local + Never (only if the image
#   is already visible to kubelet on the scheduled node).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS=k8s-ui
RELEASE=k8s-ui
THIS_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Checking kubectl context (expect docker-desktop)"
CTX="$(kubectl config current-context 2>/dev/null || true)"
if [[ "${CTX}" != "docker-desktop" ]] && [[ "${SKIP_CONTEXT_CHECK:-}" != "1" ]]; then
  echo "Error: current kubectl context is '${CTX}', expected 'docker-desktop'."
  echo "Run: kubectl config use-context docker-desktop"
  echo "Or override: SKIP_CONTEXT_CHECK=1 $0"
  exit 1
fi

NODES="$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"
USE_NEVER="${USE_LOCAL_IMAGE_NEVER:-0}"
if [[ "${NODES}" != "1" ]] && [[ "${ALLOW_MULTI_NODE:-}" != "1" ]] && [[ "${USE_NEVER}" == "1" ]]; then
  echo ""
  echo "Warning: this cluster has ${NODES} nodes."
  echo "With imagePullPolicy:Never the image must exist on the node where the"
  echo "pod is scheduled. Re-run with: ALLOW_MULTI_NODE=1 $0"
  echo "Or omit USE_LOCAL_IMAGE_NEVER (default ttl.sh flow works on multi-node)."
  echo ""
  exit 1
fi

echo "==> Building image k8s-ui:local"
docker build --no-cache -t k8s-ui:local "${ROOT}"

HELM_EXTRA_SET=()
HELM_EXTRA_FILES=(-f "${THIS_DIR}/values.yaml")

if [[ "${USE_NEVER}" == "1" ]]; then
  echo "==> Using local image k8s-ui:local (imagePullPolicy: Never)"
  HELM_EXTRA_FILES+=(-f "${THIS_DIR}/values.never-pull.yaml")
else
  # ttl.sh: tag suffix is the TTL (e.g. 8h). Image is anonymously pullable over HTTPS.
  TTL_SUFFIX="${TTL_SH_TTL:-8h}"
  RAND="$(openssl rand -hex 10)"
  TTL_IMAGE="ttl.sh/k8s-ui-${RAND}:${TTL_SUFFIX}"
  echo "==> Pushing to ${TTL_IMAGE} (ephemeral; dev-only, reachable from cluster over HTTPS)"
  docker tag k8s-ui:local "${TTL_IMAGE}"
  docker push "${TTL_IMAGE}"
  REPO="ttl.sh/k8s-ui-${RAND}"
  HELM_EXTRA_SET+=(--set-string "image.repository=${REPO}" --set-string "image.tag=${TTL_SUFFIX}" --set-string "image.pullPolicy=Always")
fi

echo "==> Helm install/upgrade ${RELEASE} in namespace ${NS}"
helm upgrade --install "${RELEASE}" "${ROOT}/deploy/helm" \
  --namespace "${NS}" \
  --create-namespace \
  "${HELM_EXTRA_FILES[@]}" \
  "${HELM_EXTRA_SET[@]}"

echo "==> Waiting for workloads"
if ! kubectl rollout status deployment/"${RELEASE}" -n "${NS}" --timeout=600s; then
  echo ""
  echo "=== Deployment rollout failed ==="
  kubectl get pods -n "${NS}" -o wide 2>/dev/null || true
  kubectl describe pod -n "${NS}" -l app.kubernetes.io/name=k8s-ui 2>/dev/null | tail -40 || true
  echo ""
  echo "If ImagePullBackOff: check outbound HTTPS (ttl.sh) or try again."
  echo "If you must use a purely local image on the node: USE_LOCAL_IMAGE_NEVER=1 $0"
  echo "(only works when kubelet can already see k8s-ui:local.)"
  echo ""
  exit 1
fi
kubectl rollout status statefulset/k8s-ui-postgres -n "${NS}" --timeout=600s 2>/dev/null || true

PF_PORT="${PF_PORT:-8080}"
echo ""
echo "Deployment finished."
echo ""
echo "Docker Desktop often does not expose NodePort on localhost reliably."
echo "Use port-forward (recommended):"
echo "  kubectl port-forward -n ${NS} svc/${RELEASE} ${PF_PORT}:80"
echo "Then open: http://127.0.0.1:${PF_PORT}/"
echo ""
echo "Sign in with token: devtoken"
echo ""
if [[ "${USE_NEVER}" != "1" ]]; then
  echo "Note: app image was pushed to ttl.sh (public URL, TTL ${TTL_SUFFIX:-8h})."
  echo "      For offline / no-registry: set USE_LOCAL_IMAGE_NEVER=1 (advanced)."
fi
