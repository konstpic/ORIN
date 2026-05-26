#!/usr/bin/env bash
# Deploy orin to Docker Desktop Kubernetes (context docker-desktop).
# Prerequisites: Docker Desktop with Kubernetes enabled, helm, kubectl.
#
# Builds three component images (apiserver, controller, reposerver). Default:
#   push each to ttl.sh (ephemeral public URL), Helm uses imagePullPolicy: Always.
#
# Optional:
#   USE_DOCKERHUB=1 — build, push to Docker Hub, deploy (DOCKERHUB_USER, DOCKERHUB_TAG)
#   USE_GHCR=1      — pull from ghcr.io (private; needs GHCR_TOKEN)
#   USE_LOCAL_IMAGE_NEVER=1 — orin-{component}:local + Never (only if
#   images are already visible to kubelet on the scheduled node).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS=orin
RELEASE=orin
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

COMPONENTS=(apiserver controller reposerver)
HELM_EXTRA_SET=()
HELM_EXTRA_FILES=(-f "${THIS_DIR}/values.yaml")

if [[ "${USE_DOCKERHUB:-0}" == "1" ]]; then
  DOCKERHUB_USER="${DOCKERHUB_USER:-konstpic}"
  CHART_TAG="${DOCKERHUB_TAG:-0.2.2}"
  echo "==> Build and push to Docker Hub (${DOCKERHUB_USER}/orin-*:${CHART_TAG})"
  HELM_EXTRA_SET+=(--set-string "global.image.tag=${CHART_TAG}")
  HELM_EXTRA_SET+=(--set-string "global.images.apiserver.repository=${DOCKERHUB_USER}/orin-apiserver")
  HELM_EXTRA_SET+=(--set-string "global.images.controller.repository=${DOCKERHUB_USER}/orin-controller")
  HELM_EXTRA_SET+=(--set-string "global.images.reposerver.repository=${DOCKERHUB_USER}/orin-reposerver")

  if ! docker info >/dev/null 2>&1; then
    echo "Error: docker is not running. Start Docker Desktop and log in to Docker Hub."
    exit 1
  fi

  for c in "${COMPONENTS[@]}"; do
    HUB_IMAGE="${DOCKERHUB_USER}/orin-${c}:${CHART_TAG}"
    echo "==> Building ${HUB_IMAGE} (target ${c})"
    docker build --target "${c}" --build-arg "VERSION=${CHART_TAG}" -t "${HUB_IMAGE}" "${ROOT}"
    echo "==> Pushing ${HUB_IMAGE}"
    docker push "${HUB_IMAGE}"
  done

  if [[ "${DOCKERHUB_SKIP_SECRET:-0}" != "1" ]] && [[ -n "${DOCKERHUB_TOKEN:-}" ]]; then
    kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    kubectl create secret docker-registry dockerhub-credentials \
      --namespace "${NS}" \
      --docker-server=https://index.docker.io/v1/ \
      --docker-username="${DOCKERHUB_USER}" \
      --docker-password="${DOCKERHUB_TOKEN}" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    HELM_EXTRA_SET+=(--set-json 'global.imagePullSecrets=[{"name":"dockerhub-credentials"}]')
    echo "==> dockerhub-credentials secret updated"
  fi
elif [[ "${USE_GHCR:-0}" == "1" ]]; then
  CHART_TAG="${GHCR_TAG:-0.2.2}"
  echo "==> Deploy from GHCR (tag ${CHART_TAG})"
  HELM_EXTRA_FILES+=(-f "${THIS_DIR}/values.ghcr.yaml")
  HELM_EXTRA_SET+=(--set-string "global.image.tag=${CHART_TAG}")

  if [[ -z "${GHCR_TOKEN:-}" ]]; then
    GHCR_TOKEN="$(printf 'protocol=https\nhost=github.com\n\n' | git credential fill 2>/dev/null | awk -F= '/^password=/{print $2; exit}')"
  fi
  if [[ -z "${GHCR_TOKEN:-}" ]]; then
    echo "Error: GHCR images are private. Set GHCR_TOKEN or run: docker login ghcr.io"
    exit 1
  fi
  GHCR_USER="${GHCR_USER:-$(git config github.user 2>/dev/null || git config user.name 2>/dev/null || echo konstpic)}"
  echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USER}" --password-stdin >/dev/null
  kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl create secret docker-registry ghcr-credentials \
    --namespace "${NS}" \
    --docker-server=ghcr.io \
    --docker-username="${GHCR_USER}" \
    --docker-password="${GHCR_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  echo "==> ghcr-credentials secret updated in ${NS}"
else
for c in "${COMPONENTS[@]}"; do
  echo "==> Building orin-${c}:local (target ${c})"
  docker build --no-cache --target "${c}" -t "orin-${c}:local" "${ROOT}"
done

if [[ "${USE_NEVER}" == "1" ]]; then
  echo "==> Using local component images (imagePullPolicy: Never)"
  HELM_EXTRA_FILES+=(-f "${THIS_DIR}/values.never-pull.yaml")
else
  TTL_SUFFIX="${TTL_SH_TTL:-8h}"
  RAND="$(openssl rand -hex 10)"
  echo "==> Pushing component images to ttl.sh (TTL ${TTL_SUFFIX}, dev-only)"
  for c in "${COMPONENTS[@]}"; do
    REPO="ttl.sh/orin-${c}-${RAND}"
    TTL_IMAGE="${REPO}:${TTL_SUFFIX}"
    docker tag "orin-${c}:local" "${TTL_IMAGE}"
    docker push "${TTL_IMAGE}"
    HELM_EXTRA_SET+=(--set-string "global.images.${c}.repository=${REPO}")
    HELM_EXTRA_SET+=(--set-string "global.images.${c}.tag=${TTL_SUFFIX}")
  done
  HELM_EXTRA_SET+=(--set-string "global.image.tag=${TTL_SUFFIX}" --set-string "global.image.pullPolicy=Always")
fi
fi

echo "==> Helm install/upgrade ${RELEASE} in namespace ${NS}"
helm upgrade --install "${RELEASE}" "${ROOT}/deploy/helm" \
  --namespace "${NS}" \
  --create-namespace \
  "${HELM_EXTRA_FILES[@]}" \
  "${HELM_EXTRA_SET[@]}"

echo "==> Waiting for workloads"
if ! kubectl rollout status deployment/"${RELEASE}-reposerver" -n "${NS}" --timeout=600s; then
  echo ""
  echo "=== reposerver rollout failed ==="
  kubectl get pods -n "${NS}" -o wide 2>/dev/null || true
  kubectl describe pod -n "${NS}" -l app.kubernetes.io/component=reposerver 2>/dev/null | tail -40 || true
  echo ""
  exit 1
fi
kubectl rollout status deployment/"${RELEASE}-controller" -n "${NS}" --timeout=600s 2>/dev/null || true
if ! kubectl rollout status deployment/"${RELEASE}-apiserver" -n "${NS}" --timeout=600s; then
  echo ""
  echo "=== apiserver rollout failed ==="
  kubectl get pods -n "${NS}" -o wide 2>/dev/null || true
  kubectl describe pod -n "${NS}" -l app.kubernetes.io/component=apiserver 2>/dev/null | tail -40 || true
  echo ""
  echo "If ImagePullBackOff: check outbound HTTPS (ttl.sh) or try again."
  echo "If you must use purely local images on the node: USE_LOCAL_IMAGE_NEVER=1 $0"
  echo "(only works when kubelet can already see orin-{apiserver,controller,reposerver}:local.)"
  echo ""
  exit 1
fi
kubectl rollout status statefulset/"${RELEASE}-postgres" -n "${NS}" --timeout=600s 2>/dev/null || true

PF_PORT="${PF_PORT:-8080}"
echo ""
echo "Deployment finished."
echo ""
echo "Docker Desktop often does not expose NodePort on localhost reliably."
echo "Use port-forward (recommended):"
echo "  kubectl port-forward -n ${NS} svc/${RELEASE}-apiserver ${PF_PORT}:80"
echo "Then open: http://127.0.0.1:${PF_PORT}/"
echo ""
echo "Sign in with token: devtoken"
echo ""
if [[ "${USE_DOCKERHUB:-0}" == "1" ]]; then
  echo "Images: ${DOCKERHUB_USER:-konstpic}/orin-{apiserver,controller,reposerver}:${DOCKERHUB_TAG:-0.2.2}"
elif [[ "${USE_GHCR:-0}" == "1" ]]; then
  echo "Images: ghcr.io/konstpic/orin-{apiserver,controller,reposerver}:${GHCR_TAG:-0.2.2}"
elif [[ "${USE_NEVER}" != "1" ]]; then
  echo "Note: component images were pushed to ttl.sh (public URLs, TTL ${TTL_SUFFIX:-8h})."
  echo "      For offline / no-registry: set USE_LOCAL_IMAGE_NEVER=1 (advanced)."
  echo "      For Docker Hub: USE_DOCKERHUB=1 $0"
  echo "      For GHCR: USE_GHCR=1 $0"
fi
