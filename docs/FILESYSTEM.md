# Filesystem Layout & Local State Strategy

## Production layout (default)

| Path | Purpose |
|------|---------|
| `/etc/releasepanel-agent/config.yaml` | Static configuration (central URL, timers, paths). |
| `/var/lib/releasepanel-agent/` | **State root** — all mutable agent data. |
| `/var/log/releasepanel-agent/` | Rotated logs; agent may write `agent.log` and structured JSON lines. |

State root subdirectories:

```text
/var/lib/releasepanel-agent/
  enrollment.json          # 0600 — credentials (see ENROLLMENT.md)
  runtime.json             # agent runtime counters / last successful loops
  inventory.json           # last published inventory snapshot (optional cache)
  deploy/
    staging/               # per-run staging dirs
    runs/                  # JSONL run records
  locks/                   # coarse flock files (single-node)
  outbox/                  # optional queued reports for retry
```

## Environment overrides

| Variable | Effect |
|----------|--------|
If `RELEASEPANEL_CONFIG` points at a YAML file, **`central_base_url` must be origin-only** (`https://host`, no `/api` path). The agent calls **`/api/v1/...`** on Central; see [CENTRAL_API.md](CENTRAL_API.md).
| `RELEASEPANEL_STATE_DIR` | Replaces `/var/lib/releasepanel-agent`. |
| `RELEASEPANEL_LOG_DIR` | Replaces `/var/log/releasepanel-agent`. |

## Local state strategy

1. **No embedded database** in v1 — SQLite or etcd are intentionally avoided.
2. **Atomic writes**: write to `*.tmp` in same directory, `fsync`, rename to final name.
3. **Schema versioning**: every JSON document includes `schema_version` (integer).
4. **Single writer**: one agent process per node; locks guard deploy runs.
5. **Inspectable**: operators can `cat` JSON state for support.

## Logs

- Human: `agent.log` (plain text, prefixed timestamps).
- Structured: optional `events.jsonl` for machine parsing (scaffold).

Rotation is delegated to **logrotate** or journald on the appliance; agent does not implement rotation in v1.

## nginx / PHP runtime files (managed later)

Application-owned configs under e.g. `/etc/nginx/sites-enabled/` are **not** modified until deploy manifests declare paths; agent adapters only expose **check/reload** operations in this scaffold.
