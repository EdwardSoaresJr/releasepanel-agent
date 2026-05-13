# Git site pull (agent scaffold)

Central may include **`site_repository_deploy_intents`** on each `GET`/`POST` **`/api/v1/agent/ping`** response for sites with in-flight repository deploy states (`requested` … `applying`).

Each intent includes **`git_ssh_url`**, **`branch`**, **`deploy_path`**, **`site_ulid`**, optional **`desired_commit_sha`**, and **`repository_deploy_key_configured`** (public key registered on GitHub — the operator still installs the read-only private key on the node).

The agent should:

1. Honor **pull-only** semantics (`git fetch` / `git reset --hard origin/<branch>`), never push.
2. Advance **`repository_deploy_state`** on the existing **`site_runtime_reports`** heartbeat rows together with TLS/runtime fields (see ReleasePanel Central `docs/GITHUB_DEPLOY_CONVERGENCE.md` and `docs/SITES_AND_TLS.md`).
3. Report **observed commit** fields when the tree identity is known (`observed_commit_sha`, message, author, timestamps).

This repository does not yet implement automatic git operations in the Go binary; Central and docs define the contract first.
