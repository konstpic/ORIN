# REST + WebSocket API

The full machine-readable spec lives at
[`internal/api/openapi.yaml`](../internal/api/openapi.yaml) and is served by
the running API server at `GET /api/openapi.yaml`.

Auth: Bearer token (`Authorization: Bearer <token>`) or HttpOnly cookie
(`k8sui-token`). The login endpoint sets the cookie.

## REST highlights

| Method | Path                                          | Description                          |
|--------|-----------------------------------------------|--------------------------------------|
| GET    | `/healthz`, `/readyz`                         | Liveness/readiness                   |
| POST   | `/api/v1/auth/login`                          | Exchange static token for a cookie   |
| GET    | `/api/v1/applications`                        | List applications                    |
| POST   | `/api/v1/application-batches`                 | Create many apps from a template + per-row overrides |
| GET    | `/api/v1/applications/{name}`                 | Get application                      |
| PUT    | `/api/v1/applications/{name}`                 | Update application                   |
| DELETE | `/api/v1/applications/{name}`                 | Delete application                   |
| POST   | `/api/v1/applications/{name}/sync`            | Enqueue a sync                       |
| POST   | `/api/v1/applications/{name}/refresh`         | Force a status reconcile             |
| GET    | `/api/v1/applications/{name}/manifests`       | Rendered desired YAML                |
| GET    | `/api/v1/applications/{name}/diff`            | Per-resource diff                    |
| GET    | `/api/v1/applications/{name}/resource-tree`   | Live resource forest                 |
| GET    | `/api/v1/applications/{name}/history`         | Recent sync operations               |
| GET    | `/api/v1/applications/{name}/events` (WS)     | Multiplexed event stream             |
| GET/POST/DELETE | `/api/v1/repositories[...]`          | Repo registration                    |
| GET    | `/api/v1/clusters`                            | List clusters                        |

## WebSocket protocol

The connection is multiplexed. Client sends frames like:

```json
{ "action": "subscribe",   "topic": "app:web:status" }
{ "action": "unsubscribe", "topic": "app:web:status" }
```

Server pushes frames in this shape:

```json
{ "topic": "app:web:status", "type": "status", "payload": { /* AppStatus */ } }
```

Built-in topics:

- `app:<name>:status` — status row updates.
- `app:<name>:sync`   — sync operation lifecycle.
