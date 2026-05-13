# Nginx runtime convergence (bounded termination)

This document describes **bounded nginx materialization and reload convergence** in **releasepanel-agent**: explicit argv only, site-scoped files only, and receipts on **`POST /api/v1/agent/ping`** via **`site_runtime_reports`** (`runtime_apply_state`, `runtime_reload_observed_at`, `failure_reason`).

Cross-read Central doctrine:

- **Central** `docs/RELEASEPANEL_OPERATING_PHILOSOPHY.md`
- **Central** `docs/RUNTIME_APPLY_CONVERGENCE.md`
- **Central** `docs/GITHUB_DEPLOY_CONVERGENCE.md` (repository slice — orthogonal)
- **Central** `docs/SITES_AND_TLS.md` (TLS + ping row shape)

## Philosophy

| Rule | Meaning |
|------|---------|
| Inspectable | Fixed template text per runtime type; deterministic paths (`rp-<site_ulid>.conf`). |
| Observation-first | Central advances only on valid transitions from agent receipts — no invented “healthy”. |
| Bounded mutation | Writes only under configured **`nginx_sites_available_root`** and symlinks only under **`nginx_sites_enabled_root`**. |
| Explicit execution | **`nginx_test_argv`** (`nginx -t`) and **`nginx_reload_argv`** (`nginx -s reload` or `systemctl reload nginx`) — no shell interpolation. |

## Ping integration

| Step | Mechanism |
|------|-----------|
| Observe intent | **`GET /api/v1/agent/ping`** returns **`site_runtime_apply_intents`** for sites in **`requested`**, **`applying`**, or **`reload_requested`**. |
| Receipt | **`POST /api/v1/agent/ping`** with **`site_runtime_reports`** rows: **`tls_state`** + **`observed_at`** (required), optional **`runtime_apply_state`**, **`runtime_reload_observed_at`**, **`failure_reason`**. |

Central maps **`failure_reason`** into runtime apply failure fields when **`runtime_apply_state`** is **`failed`** (shared key with TLS failures on the same row — keep receipts scoped per concern).

TLS echo persistence (**`site_tls_echo.json`**) stays compatible with ping validation; see [GITHUB_SITE_PULL.md](GITHUB_SITE_PULL.md).

## Materialization boundaries

- **Available**: atomic write `nginx_sites_available_root/rp-<site_ulid>.conf` (mode **0644**).
- **Enabled**: symlink `nginx_sites_enabled_root/rp-<site_ulid>.conf` → absolute path of the available file.
- **No** edits outside those filenames; **no** traversal; roots must be absolute and cannot be filesystem **`/`**.

Ledger **`deploy_path`** and derived **web root** must remain under **`runtime_deploy_path_roots`** (same bounded roots as Git convergence).

## Apply vs reload legs

| Ledger `runtime_apply_state` | Agent behavior |
|------------------------------|----------------|
| **`requested`** | Receipt **`applying`** → materialize → **`nginx -t`** → receipt **`applied`** (or **`failed`**). |
| **`applying`** | Skip duplicate **`applying`** receipt → materialize → **`nginx -t`** → **`applied`** / **`failed`**. |
| **`reload_requested`** | Rematerialize (idempotent) → **`nginx -t`** → **`nginx_reload_argv`** → **`reload_applied`** + **`runtime_reload_observed_at`** (or **`failed`**). |

Runtime work may repeat; receipts do not move backwards (same pattern as Git deploy convergence).

## Runtime types (initial)

Deterministic **`try_files`** / PHP handling only — **no** template marketplace or DSL.

| `runtime_type` | Web root | PHP |
|----------------|----------|-----|
| **`laravel`** | `deploy_path/public` | yes (`fastcgi_pass`) |
| **`wordpress`** / **`wp`** | `deploy_path` | yes |
| **`php`** | `deploy_path` | yes |
| **`static`** | `deploy_path` | no |
| **`custom`** | `deploy_path` | yes (minimal PHP block) |

**`nginx_php_fastcgi_pass`** defaults to `unix:/run/php/php8.3-fpm.sock` when unset; required implicitly whenever PHP is included.

## Configuration (agent)

See [config.example.yaml](../config/config.example.yaml):

- **`nginx_sites_available_root`** / **`nginx_sites_enabled_root`** (both set or both empty)
- **`nginx_test_argv`**, **`nginx_reload_argv`** (optional defaults)
- **`nginx_php_fastcgi_pass`**

Operator must ensure the main **`nginx.conf`** includes the enabled directory (ReleasePanel does not rewrite global nginx).

## Explicit non-goals

- Arbitrary nginx snippets, generators, or “marketplace” configs  
- Shell pipelines, deploy scripts, CI/CD, containers  
- Service supervision, watchdog restart loops, synthetic health narratives  

## Related

- [CENTRAL_API.md](CENTRAL_API.md) — ping + intents  
- [GITHUB_SITE_PULL.md](GITHUB_SITE_PULL.md) — TLS echo + Git receipts  
- [TLS_RUNTIME_CONVERGENCE.md](TLS_RUNTIME_CONVERGENCE.md) — bounded TLS termination materialization (`listen 443 ssl`, ledger PEM paths)  
