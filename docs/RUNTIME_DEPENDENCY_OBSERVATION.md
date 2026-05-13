# Runtime dependency observation

This document describes **bounded runtime dependency evidence** emitted by **releasepanel-agent** on **`POST /api/v1/agent/ping`**. It is **not** a monitoring stack, health score, uptime product, or observability pipeline.

## Philosophy

- **Observation outranks optimism** — probes record what the boundary saw, not intent.
- **Runtime stays inspectable** — explicit probes (socket stat + dial, bounded TCP connect, optional fixed `argv`, optional pid file) are reviewable in code.
- **Heartbeat is the receipt** — observations ride the same Sanctum ping channel as other bounded receipts.
- **Agent remains bounded** — no auto-discovery, no shell scripts, no plugin system.
- **No synthetic “healthy”** — a successful probe means only that **the configured dependency responded as expected at the boundary** (e.g. TCP connect succeeded, unix socket accepted a dial). It does **not** mean the application, deployment, or host is “healthy”.

Cross-read:

- Central **`docs/RELEASEPANEL_OPERATING_PHILOSOPHY.md`** — operating posture.
- [CONVERGENCE.md](CONVERGENCE.md) — convergence framing on the agent (shared vocabulary with TLS/nginx/Git receipts).
- [NGINX_RUNTIME_CONVERGENCE.md](NGINX_RUNTIME_CONVERGENCE.md) — nginx materialization receipts.
- [TLS_RUNTIME_CONVERGENCE.md](TLS_RUNTIME_CONVERGENCE.md) — TLS ledger/runtime receipts.

## Evidence-only semantics

Each probe produces one row in **`runtime_dependency_reports`**:

| Field | Meaning |
|-------|---------|
| **`dependency`** | Configured dependency name: `php_fpm`, `redis`, `mysql`, `postgres`. |
| **`state`** | `observed` \| `missing` \| `unreachable` \| `failed` (same vocabulary family as other convergence evidence — **not** “healthy/degraded/green”). |
| **`observed_at`** | RFC3339 timestamp when the observation was taken. |
| **`failure_reason`** | Optional terse boundary reason (e.g. dial error, missing path, `systemctl` non-zero). Absent when `observed`. |

**States (bounded):**

- **`observed`** — every configured check for that row succeeded at the boundary (e.g. socket is a socket node and accepted a dial; TCP connect completed; optional `systemctl` exited 0; optional pid file + live pid check succeeded).
- **`missing`** — expected local artifact absent or inactive (e.g. socket path missing, `systemctl is-active` non-zero semantics, pid file missing or pid not running).
- **`unreachable`** — dependency appears present but did not accept the bounded probe within limits (e.g. TCP refused/timeout, unix dial timeout).
- **`failed`** — probe misconfiguration or unexpected probe error (e.g. non-socket file at socket path, invalid tcp address, exec failure).

Central stores the latest validated snapshot under **`runtime_meta.runtime_dependency_reports`** (bounded array, no rollups, no aggregate health flag).

## Configuration (explicit only)

In **`config.yaml`**:

```yaml
runtime_dependency_probes:
  - type: php_fpm
    socket: /run/php/php8.3-fpm.sock
    # optional explicit argv (no shell):
    # systemctl_argv:
    #   - /usr/bin/systemctl
    #   - is-active
    #   - --quiet
    #   - php8.3-fpm.service
  - type: redis
    tcp: 127.0.0.1:6379
  - type: mysql
    tcp: 127.0.0.1:3306
  - type: postgres
    tcp: 127.0.0.1:5432
```

Rules:

- **`php_fpm`** requires **`socket`** (absolute unix path). Must not set **`tcp`**.
- **`redis`**, **`mysql`**, **`postgres`** require **`tcp`** as **`host:port`**. Must not set **`socket`**.
- Up to **32** probes.
- Optional **`systemctl_argv`**: non-empty argv slice; index **`0`** must be an **absolute** executable path; bounded argument count/length.
- Optional **`pid_file`**: absolute path; pid must respond to **signal 0** on Unix (process existence check).

No scanning, no inference from packages listening — only configured targets.

## Non-goals

- Health scores, SLIs, dashboards, charts, metrics streaming, alerting, supervisors, or self-healing.
- Declaring “all services operational” or application-level health from these probes.

## Logging

The agent logs terse lines when a probe is **not** `observed`, e.g. `redis probe unreachable`. It does **not** log synthetic “runtime healthy” summaries.
