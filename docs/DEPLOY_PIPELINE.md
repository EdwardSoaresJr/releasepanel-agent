# Deploy Execution Pipeline (Architecture)

The deploy runner is an **explicit finite-state pipeline**. Each deployment attempt produces a **run record** on disk (append-only log file + optional JSON snapshot). No dynamic plugin loading in v1.

## Lifecycle states

```text
 IDLE ──► FETCH_DESIRED ──► VALIDATE ──► PREPARE ──► APPLY ──► VERIFY ──► REPORT ──► IDLE
                                    ╲___FAIL___╱         ╲___FAIL___╱
```

| Phase | Purpose |
|-------|---------|
| **FETCH_DESIRED** | Pull desired manifest/spec from central (or local mirror path for testing). Record `desired_fingerprint`. |
| **VALIDATE** | Schema + local capability checks (paths allowed, required binaries present). Fail fast. |
| **PREPARE** | Staging directory, extract artifacts, render templates **later**; v1 may noop beyond directories. |
| **APPLY** | Ordered steps: nginx config sync, php pool sync, reload commands — **explicit shell hooks later**; scaffold registers intended commands only. |
| **VERIFY** | `nginx -t`, php-fpm ping/socket check, HTTP probe optional later. |
| **REPORT** | POST convergence + step outcomes to central; write local run record. |

## Determinism rules

- Steps run in **declaration order** from the manifest (central-defined order wins).
- Same manifest fingerprint → same ordered steps (idempotent apply where possible).
- Timeouts and retry caps are **config constants**, not implicit.

## Future work (explicitly out of scope now)

- Git pull, Composer, migrations — invoked only via declared steps when added.
- Blue/green, rolling — higher-level central concern; agent executes one manifest at a time.

## Artifact locations (see FILESYSTEM.md)

- Staging: `{state_dir}/deploy/staging/<run_id>/`
- Records: `{state_dir}/deploy/runs/<run_id>.jsonl`
