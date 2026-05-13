# Central HTTP API (agent contract)

This document is the **integration contract** between **releasepanel-agent** and **releasepanel-central**. Treat it as normative for paths and URL shaping.

## URL convention (locked)

| Piece | Rule |
|-------|------|
| **`central_base_url`** in agent config | **Origin only**: `https://releasepanel.com` or `https://releasepanel.com:8443`. **No path**, **no** trailing `/api`, **no** query string. |
| **HTTP routes on Central** | Always under **`/api/v1/...`** relative to that origin. |

Examples:

| `central_base_url` | First enroll request | Correct? |
|--------------------|----------------------|----------|
| `https://releasepanel.com` | `POST https://releasepanel.com/api/v1/nodes/enroll` | Yes |
| `https://releasepanel.com/api` | Agent appends `/api/v1/...` → **`.../api/api/v1/...`** | **No** — config validation rejects non-empty paths |

So Central continues to mount its router at **`/api/v1`**; the agent never folds `/api` into the base URL.

## Authenticated requests

After enrollment, the agent sends:

```http
Authorization: Bearer <api_key>
X-Releasepanel-Node-Id: <node_id>
```

Enrollment uses **no** bearer token (only JSON body with enrollment token).

## Endpoints the agent calls today

| Method | Path | Auth | Body (JSON) | Purpose |
|--------|------|------|-------------|---------|
| `POST` | `/api/v1/nodes/enroll` | No | [`EnrollRequest`](../pkg/api/types.go) | Issue node credentials |
| `POST` | `/api/v1/nodes/{id}/heartbeat` | Yes | [`HeartbeatReport`](../pkg/api/types.go) | Liveness / `last_seen_at` spine |
| `POST` | `/api/v1/nodes/{id}/reports/inventory` | Yes | [`InventoryReport`](../pkg/api/types.go) | Slow-changing snapshot |
| `POST` | `/api/v1/nodes/{id}/reports/health` | Yes | [`HealthReport`](../pkg/api/types.go) | Probe summary |
| `GET` | `/api/v1/nodes/{id}/desired` | Yes | — (JSON manifest) | Desired manifest fetch (**only when** `manifest_reconcile_enabled: true`) |
| `POST` | `/api/v1/nodes/{id}/reports/convergence` | Yes | [`ConvergenceReport`](../pkg/api/types.go) | Deploy/convergence status (**reconcile enabled**) |
| `GET` | `/api/v1/agent/ping` | Bearer only (Sanctum **agent** token) | — | Observe **`site_repository_deploy_intents`** + **`site_runtime_apply_intents`** (**when** `agent_ping_token_file` configured) |
| `POST` | `/api/v1/agent/ping` | Bearer only (Sanctum **agent** token) | [`AgentPingPostBody`](../pkg/api/ping.go) (`site_runtime_reports` receipts; optional **`runtime_dependency_reports`** bounded dependency observations) | Bounded Git / nginx runtime convergence + TLS/runtime observations + optional runtime dependency evidence |

`{id}` is URL-encoded `node_id`.

**Ping auth** uses **`Authorization: Bearer <token>`** from **`POST /api/v1/agent/bootstrap`** (no `X-Releasepanel-Node-Id` header). Persist the token path via **`agent_ping_token_file`** in [config](../internal/config/config.go). When set, **`runtime_deploy_path_roots`** must list allowed absolute prefixes for site **`deploy_path`** values.

See [GITHUB_SITE_PULL.md](GITHUB_SITE_PULL.md) for clone/fetch/reset semantics, [NGINX_RUNTIME_CONVERGENCE.md](NGINX_RUNTIME_CONVERGENCE.md) for nginx materialization + reload receipts, [TLS_RUNTIME_CONVERGENCE.md](TLS_RUNTIME_CONVERGENCE.md) for TLS receipts, and [RUNTIME_DEPENDENCY_OBSERVATION.md](RUNTIME_DEPENDENCY_OBSERVATION.md) for bounded dependency probes on ping POST.

## Migrating stored `central_base_url`

Older installs may have persisted **`https://host/api`** inside `enrollment.json`. The agent now rejects any non-empty path on `central_base_url`. Fix by setting **`central_base_url`** to the origin only in config **and** editing **`enrollment.json`** `central_base_url` field to match, or re-enroll.

## Spine-first rollout (recommended)

1. Implement on Central: **enroll**, **heartbeat**, **inventory**, **health** (store payloads + timestamps such as `last_seen_at`).
2. Run agent **`run`** with **`manifest_reconcile_enabled: false`** (default).
3. Confirm node appears with heartbeat + inventory + health in Central (API or admin UI).
4. Implement **GET desired** + convergence reporting on Central, then set **`manifest_reconcile_enabled: true`** on the node.

## Related docs

- [Operational doctrine](OPERATIONS.md)
- [Enrollment flow](ENROLLMENT.md)
- [Architecture overview](ARCHITECTURE.md)
- [Nginx runtime convergence](NGINX_RUNTIME_CONVERGENCE.md)
- [TLS runtime convergence](TLS_RUNTIME_CONVERGENCE.md)
