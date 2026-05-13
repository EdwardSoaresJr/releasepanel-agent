# GitHub site pull (bounded runtime convergence)

This document describes how **releasepanel-agent** performs **bounded Git repository convergence** against GitHub: explicit commands only, pull-only semantics, and receipts on **`POST /api/v1/agent/ping`** via **`site_runtime_reports`** (not deployment storytelling).

Cross-read Central doctrine (authoritative product semantics):

- **Central** `docs/RELEASEPANEL_OPERATING_PHILOSOPHY.md` — operating posture
- **Central** `docs/GITHUB_DEPLOY_CONVERGENCE.md` — intent vs observation, ledger
- **Central** `docs/RUNTIME_APPLY_CONVERGENCE.md` — runtime apply slice (orthogonal to repository convergence)

## Philosophy (preserved)

| Principle | Meaning |
|-----------|---------|
| GitHub is source authority | Runtime never invents ref identity; it observes the remote tree. |
| Runtime is pull-only | No push, merge/rebase/stash orchestration, or “helpful” local mutations. |
| Agent remains bounded | Fixed git argv sequences, path roots, single-purpose SSH wrapper — no shell frameworks. |
| Heartbeat is the receipt | Central advances ledger states from **ping POST** observations — not from narrative APIs. |
| Observation outranks intent | Reported **`repository_deploy_state`** and **`observed_commit_*`** fields are evidence Central may merge when valid. |
| Runtime stays inspectable | Terse logs and explicit failure strings (`repository_deploy_failure_reason`). |

## Integration surface

| Mechanism | Role |
|-----------|------|
| **`GET /api/v1/agent/ping`** | Observe **`site_repository_deploy_intents`** for this server (Sanctum agent token). |
| **`POST /api/v1/agent/ping`** | Submit **`site_runtime_reports`** rows including **`repository_deploy_state`**, optional **`repository_deploy_failure_reason`**, and **`observed_commit_*`** fields. |

Nodes API heartbeats (`POST /api/v1/nodes/{id}/heartbeat`) remain separate scaffolding; **repository deploy convergence receipts require the agent ping route** because Central validates those fields on ping.

Configure **`agent_ping_token_file`** (path to the bearer token from **`POST /api/v1/agent/bootstrap`**) and **`runtime_deploy_path_roots`** — see [CENTRAL_API.md](CENTRAL_API.md).

## Convergence flow (single slice)

Central declares deploy intent (ledger) → agent observes intent on ping → bounded git work → POST observations → Central **`SiteRepositoryDeployConvergence`** advances valid edges.

Per intent, the agent resumes from **`deploy_state`** on the intent row (Central ledger): it skips **`fetching`** / **`fetched`** receipts when the ledger is already past those phases (avoiding invalid backwards transitions). It still runs **`git fetch`** / clone while work is in flight, then **`git reset --hard`**, then emits **`applied`** with observed commit metadata.

1. Validates **`deploy_path`** against configured absolute **`runtime_deploy_path_roots`** (no empty path, no traversal semantics outside roots, no filesystem root as a target).
2. Resolves the read-only deploy private key at **`{state_dir}/repository-deploy-keys/{site_ulid}.key`** (mode **0600**, not group/world readable).
3. Emits **`fetching`** when the ledger is still in **`requested`** or **`fetching`** (skipped when already **`fetched`** / **`applying`**).
4. If the path is missing a git repo: **`git clone --branch <branch> -- <git_ssh_url> <deploy_path>`** (parent directory must already exist and satisfy roots).
5. If the repo exists: **`git fetch origin`** (no merge/rebase).
6. Emits **`fetched`** when not yet past fetch on the ledger (**skipped** when ledger is already **`applying`**), then emits **`applying`** before reset.
7. **`git reset --hard`** to **`origin/<branch>`**, or to **`desired_commit_sha`** when Central supplied it and it resolves after fetch.
8. Reads **`HEAD`** identity (`git rev-parse`, **`git log -1`** for subject/author).
9. Reports **`applied`** with **`observed_commit_sha`**, **`observed_commit_message`**, **`observed_commit_author`**, **`observed_commit_observed_at`**.

That is the entire repository convergence slice — **no** Composer, npm, migrations, deploy scripts, webhooks, Actions, or pipelines.

## SSH deploy key handling

The operator installs Central’s **read-only** GitHub deploy key private material on disk. The agent sets a **temporary per-invocation** environment only:

- **`GIT_SSH_COMMAND=ssh -i <key> -o IdentitiesOnly=yes …`**

Optional **`github_ssh_known_hosts_file`** pins host keys (`StrictHostKeyChecking=yes`). If omitted, **`StrictHostKeyChecking=accept-new`** is used for a bounded first-connect bootstrap (document operational trade-off).

No global `~/.ssh/config` mutation and no agent-wide credential bleed beyond this subprocess environment.

## Ping payload: TLS echo

Central requires **`tls_state`** and **`observed_at`** on every **`site_runtime_reports`** row. The agent persists **`site_tls_echo.json`** (`site_ulid` → last echoed **`tls_state`**, default **`none`**) so repository receipts remain compatible without inventing TLS observations.

## **`repository_deploy_state` semantics**

Allowed observation states for this slice:

| State | Meaning |
|-------|---------|
| **`fetching`** | Fetch/clone phase started. |
| **`fetched`** | **`git fetch`** completed (or clone finished). |
| **`applying`** | Reset phase started. |
| **`applied`** | Tree reflects requested ref; commit observation attached. |
| **`failed`** | Bounded failure; **`repository_deploy_failure_reason`** set (Central ignores **`failed`** without a reason). |

Do **not** use product-health phrases (`healthy`, `deployed successfully`, `synced`) in agent logs or payloads — keep language operational.

## Observed commit fields

These are **runtime observation evidence**, not deployment narratives:

- **`observed_commit_sha`**
- **`observed_commit_message`** (subject line, bounded)
- **`observed_commit_author`**
- **`observed_commit_observed_at`**

## Explicit non-goals

- CI/CD, workflow engines, or release orchestration theatre
- Composer / npm / arbitrary shell deploy scripts
- Webhooks or GitHub Actions integration
- “Deployment successful” UX strings on the agent

## Related

- [CENTRAL_API.md](CENTRAL_API.md) — ping + auth notes
- [NGINX_RUNTIME_CONVERGENCE.md](NGINX_RUNTIME_CONVERGENCE.md) — bounded nginx materialization + reload receipts
- [TLS_RUNTIME_CONVERGENCE.md](TLS_RUNTIME_CONVERGENCE.md) — ledger PEM paths + `listen 443 ssl` materialization
- [FILESYSTEM.md](FILESYSTEM.md) — state directories including **`repository-deploy-keys/`**
