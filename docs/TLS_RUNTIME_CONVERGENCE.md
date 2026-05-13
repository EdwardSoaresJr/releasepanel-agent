# TLS runtime convergence (bounded termination)

This document describes **bounded TLS termination materialization** in **releasepanel-agent**: nginx listens and `ssl_certificate` directives driven **only** by ledger hints (paths issued outside the agent). It is **not** ACME automation, **not** certificate generation, and **not** a TLS product dashboard.

Cross-read Central doctrine (paths only — authoritative semantics live in Central):

- **Central** `docs/SITES_AND_TLS.md`
- **Central** `docs/RUNTIME_APPLY_CONVERGENCE.md`
- [NGINX_RUNTIME_CONVERGENCE.md](NGINX_RUNTIME_CONVERGENCE.md) — nginx materialization + reload vocabulary
- **Central** `docs/RELEASEPANEL_OPERATING_PHILOSOPHY.md`

## Philosophy

| Rule | Meaning |
|------|---------|
| TLS is convergence truth | Observations posted on ping receipts (`tls_state`, `failure_reason`) advance the ledger — not UI optimism. |
| No ACME in the agent | The agent **never** runs certbot, ACME clients, DNS hooks, or issuance workflows. |
| Provenance stays visible | PEM paths come from Central’s **`ssl_certificate_path`** / **`ssl_certificate_key_path`** fields on the site row — surfaced on **`site_runtime_apply_intents`**. |
| Same termination vocabulary | Write → **`nginx -t`** → reload → receipts — identical bounded pattern as HTTP-only runtime convergence. |

## Ping hints (Central → agent)

Optional fields on each **`site_runtime_apply_intents`** row:

| Field | Meaning |
|-------|---------|
| **`tls_enabled`** | `true` when both PEM paths are non-empty on the ledger. |
| **`ssl_certificate_path`** | Absolute path to PEM material on the node (operator/Central issuance out of band). |
| **`ssl_certificate_key_path`** | Absolute path to private key material on the node. |
| **`redirect_http_to_https`** | When `true`, render an explicit HTTP → HTTPS redirect `server` block; when `false`, HTTP and HTTPS app servers may coexist. |
| **`tls_ledger_state`** | Current **`tls_state`** on the site — used to emit **`tls_state: applied`** on the reload receipt when the ledger is **`issued`** (bounded transition to **`applied`** after successful reload). |

## Agent behavior

1. When **`tls_enabled`**, the agent validates PEM paths (**absolute**, **readable regular files**, **no traversal**, under **`tls_certificate_path_roots`**) **before** rendering nginx.
2. **Fixed templates only**: `listen 443 ssl`, `ssl_certificate`, `ssl_certificate_key`, optional redirect block — see **`internal/nginxruntime/render.go`**.
3. **`nginx_test_argv`** then **`nginx_reload_argv`** with explicit argv only (see [NGINX_RUNTIME_CONVERGENCE.md](NGINX_RUNTIME_CONVERGENCE.md)).
4. Failures produce **`failure_reason`** on **`site_runtime_reports`** (runtime apply leg may report **`failed`**; TLS-specific messaging remains bounded and inspectable).

### Configuration

```yaml
tls_certificate_path_roots:
  - /etc/letsencrypt
  - /etc/ssl/releasepanel
```

Required whenever an intent has **`tls_enabled: true`**. Without roots, the agent posts a bounded failure instead of silently dropping TLS.

### TLS observation on reload

After a successful reload, when **`tls_ledger_state`** is **`issued`**, the agent sets **`tls_state` → `applied`** on the same ping row as **`reload_applied`** / **`runtime_reload_observed_at`** so Central can converge TLS without inventing “HTTPS healthy” language.

## Explicit non-goals

- ACME / Let’s Encrypt orchestration inside the agent  
- Certificate renewal scheduling, DNS validation, or CA integration  
- Ingress controllers, service meshes, or TLS dashboards  
- Silent fallback to HTTP-only when TLS hints are present  

## Related

- [CENTRAL_API.md](CENTRAL_API.md) — ping payloads  
- [GITHUB_SITE_PULL.md](GITHUB_SITE_PULL.md) — TLS echo persistence for required ping rows  
