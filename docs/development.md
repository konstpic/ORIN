# Development guide

## Prerequisites

- Go ≥ 1.23
- Node.js ≥ 20
- Docker (for running Postgres locally and building images)
- A Kubernetes context (kind / minikube / docker-desktop) for live testing

## First-time setup

```bash
git clone <repo>
cd orin

# Backend deps
go mod download

# Frontend deps
cd web && npm install && cd ..

# Local Postgres
docker run -d --rm --name orin-pg -p 5432:5432 \
  -e POSTGRES_USER=k8sui -e POSTGRES_PASSWORD=k8sui -e POSTGRES_DB=k8sui \
  postgres:16-alpine
```

## Running

```bash
# Backend (single-process MVP)
DATABASE_URL=postgres://k8sui:k8sui@localhost:5432/k8sui?sslmode=disable \
ADMIN_TOKEN=devtoken \
ENCRYPTION_KEY=$(openssl rand -hex 32) \
KUBECONFIG=$HOME/.kube/config \
  go run ./cmd/orin all-in-one

# Frontend (in another terminal)
cd web && npm run dev
```

Open <http://localhost:5173> and sign in with `devtoken`.

## Testing

```bash
make test           # Go unit tests (race-enabled)
cd web && npm run typecheck
```

## Adding migrations

Add a paired `internal/store/migrations/NNNN_name.up.sql` /
`NNNN_name.down.sql`. They are embedded into the binary at build time and
applied automatically on every startup (idempotent).

## Adding a REST endpoint

1. Update `internal/api/openapi.yaml` with the new operation.
2. Add the handler under `internal/api/`.
3. Wire it into `Handler()` in `internal/api/server.go`.
4. Add the TS client method in `web/src/api/client.ts` and the DTO in
   `web/src/api/types.ts`.

## Adding a Kubernetes Kind to the resource tree

Edit `internal/k8s/tree.go` and append the GVR to `defaultDiscoveryGVRs`
(top-level) or `defaultChildGVRs` (discovered via owner refs).
