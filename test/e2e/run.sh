#!/usr/bin/env bash
# End-to-end smoke test for orin. See README.md for the steps.
set -euo pipefail

CLUSTER=orin-e2e
PG_CONT=orin-e2e-pg
API_PORT=8080
ADMIN_TOKEN=devtoken
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
TMP=$(mktemp -d)
trap 'cleanup' EXIT

cleanup() {
  set +e
  echo "--- cleanup ---"
  [[ -n "${API_PID:-}" ]] && kill "${API_PID}" 2>/dev/null
  docker rm -f "${PG_CONT}" 2>/dev/null
  kind delete cluster --name "${CLUSTER}" 2>/dev/null
  rm -rf "${TMP}"
}

req() {
  local method=$1 path=$2 body=${3:-}
  if [[ -n "${body}" ]]; then
    curl -fsSL -X "${method}" "http://localhost:${API_PORT}${path}" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" -d "${body}"
  else
    curl -fsSL -X "${method}" "http://localhost:${API_PORT}${path}" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  fi
}

wait_status() {
  local name=$1 want_sync=$2 want_health=$3 timeout=${4:-120}
  local end=$((SECONDS+timeout))
  while (( SECONDS < end )); do
    local resp
    resp=$(req GET "/api/v1/applications/${name}" || true)
    local sync health
    sync=$(echo "${resp}" | jq -r '.status.sync')
    health=$(echo "${resp}" | jq -r '.status.health')
    echo "  ${name}: sync=${sync} health=${health}"
    if [[ "${sync}" == "${want_sync}" && "${health}" == "${want_health}" ]]; then return 0; fi
    sleep 2
  done
  echo "TIMEOUT waiting for ${name} sync=${want_sync} health=${want_health}"
  return 1
}

echo "--- creating kind cluster ${CLUSTER} ---"
kind delete cluster --name "${CLUSTER}" 2>/dev/null || true
kind create cluster --name "${CLUSTER}"

echo "--- starting postgres ---"
docker run -d --rm --name "${PG_CONT}" -p 5432:5432 \
  -e POSTGRES_USER=k8sui -e POSTGRES_PASSWORD=k8sui -e POSTGRES_DB=k8sui \
  postgres:16-alpine >/dev/null
for i in $(seq 1 30); do
  docker exec "${PG_CONT}" pg_isready -U k8sui >/dev/null 2>&1 && break
  sleep 1
done

echo "--- preparing git repo at ${TMP}/repo.git ---"
REPO_DIR=${TMP}/repo
REMOTE=${TMP}/repo.git
mkdir -p "${REPO_DIR}" "${REMOTE}"
git init --bare "${REMOTE}" >/dev/null
git -C "${REPO_DIR}" init -b main >/dev/null
git -C "${REPO_DIR}" config user.email t@t.t
git -C "${REPO_DIR}" config user.name t
mkdir -p "${REPO_DIR}/app"
cat > "${REPO_DIR}/app/deployment.yaml" <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector: { matchLabels: { app: web } }
  template:
    metadata: { labels: { app: web } }
    spec:
      containers:
        - name: web
          image: nginxinc/nginx-unprivileged:1.27-alpine
          ports: [{ containerPort: 8080 }]
YAML
git -C "${REPO_DIR}" add . && git -C "${REPO_DIR}" commit -m init >/dev/null
git -C "${REPO_DIR}" remote add origin "${REMOTE}" || true
git -C "${REPO_DIR}" push -u origin main >/dev/null

echo "--- building binary ---"
(cd "${ROOT}" && go build -o "${TMP}/orin" ./cmd/orin)

echo "--- launching api ---"
KUBECONFIG=${TMP}/kubeconfig
kind get kubeconfig --name "${CLUSTER}" > "${KUBECONFIG}"
DATABASE_URL="postgres://k8sui:k8sui@localhost:5432/k8sui?sslmode=disable" \
ADMIN_TOKEN="${ADMIN_TOKEN}" \
ENCRYPTION_KEY="${ENCRYPTION_KEY}" \
KUBECONFIG="${KUBECONFIG}" \
HTTP_ADDR=":${API_PORT}" \
REPO_CACHE_DIR="${TMP}/cache" \
REPO_POLL_INTERVAL="5s" \
"${TMP}/orin" all-in-one >"${TMP}/api.log" 2>&1 &
API_PID=$!

for i in $(seq 1 40); do
  curl -fsSL "http://localhost:${API_PORT}/healthz" >/dev/null 2>&1 && break
  sleep 0.5
done
echo "  api up (pid=${API_PID})"

echo "--- registering repo + app ---"
req POST /api/v1/repositories "{\"url\":\"${REMOTE}\"}"
echo
kubectl --kubeconfig "${KUBECONFIG}" create ns demo
req POST /api/v1/applications "{
  \"name\":\"web\",
  \"source\":{\"repoUrl\":\"${REMOTE}\",\"path\":\"app\",\"targetRevision\":\"main\"},
  \"destination\":{\"cluster\":\"in-cluster\",\"namespace\":\"demo\"}
}"
echo

echo "--- initial sync ---"
req POST /api/v1/applications/web/sync ''
echo
wait_status web Synced Healthy 180

echo "--- editing git: scaling to 3 ---"
sed -i.bak 's/replicas: 1/replicas: 3/' "${REPO_DIR}/app/deployment.yaml"
git -C "${REPO_DIR}" commit -am scale >/dev/null
git -C "${REPO_DIR}" push >/dev/null
wait_status web OutOfSync Healthy 60

echo "--- second sync ---"
req POST /api/v1/applications/web/sync ''
echo
wait_status web Synced Healthy 120

echo "--- success ---"
