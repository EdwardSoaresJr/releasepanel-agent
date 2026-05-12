# Convergence & Inventory Reporting

## Convergence model

The agent maintains three fingerprints in local state:

| Field | Meaning |
|-------|---------|
| `desired_fingerprint` | Hash of last successfully **fetched** desired manifest from central. |
| `applied_fingerprint` | Hash of manifest **successfully** applied through VERIFY. |
| `last_reported_fingerprint` | Last fingerprint **acknowledged** by central (or last POST attempt baseline). |

**Converged** when:

```text
desired_fingerprint == applied_fingerprint
```

…and central has received a report including that fingerprint (within reporting semantics).

If `desired` changes, agent transitions to **pending convergence** and schedules/runs the deploy pipeline.

## Reporting payloads (conceptual)

Periodic **inventory report** (slow-changing):

- OS, kernel, arch, hostname, machine id
- nginx binary path/version (if detectable)
- PHP CLI/FPM versions (if detectable)
- Disk availability summary

Periodic **health report** (fast-changing):

- Probe results: nginx config test, php-fpm socket/ping
- Timestamps and durations

**Convergence report** (event + periodic):

- Fingerprints above; last deploy run id; phase outcomes

All bodies are **JSON** with a top-level `schema_version` field.

## Cadence

- Inventory: default **hourly** (configurable).
- Health: default **minutes** (configurable).
- Convergence/deploy: **immediate** on manifest change detection + periodic reconcile.

## Local-first

Central may be unavailable; agent:

- Retains last known desired fingerprint (if any).
- Buffers reports **best-effort** (v1: write to `outbox/` JSON files for retry — optional scaffold; minimal implementation may log-and-skip).

The codebase scaffolds report **builders** and HTTP POST stubs so central integration is additive.
