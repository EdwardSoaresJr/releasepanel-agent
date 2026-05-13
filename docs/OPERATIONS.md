# Operational doctrine — source authority vs runtime

ReleasePanel separates **who may publish history** from **who may consume and apply** it. This document states the boundary so deployments stay explainable and runtimes stay inspectable.

## Pull-only on the VPS

The production host **pulls** from Git (`fetch` / reset or equivalent to an observed branch/commit). It does **not** `git push`.

| If the VPS could push | What blurs |
|------------------------|------------|
| Git becomes a second publisher | **Provenance** (what actually shipped) |
| Server-side commits | **Authorship** and review discipline |
| Mutable repo state from runtime | **Runtime identity** vs declared releases |
| Unclear “who moved the needle” | **Deployment semantics** and rollback story |

**Pull-only** preserves **runtime legibility**: the filesystem reflects an observable git ref that traces back to GitHub.

## Layer responsibilities

| Layer | Responsibility |
|-------|------------------|
| **GitHub** | Canonical **source truth** — commits, history, PRs, tags. |
| **Laptop / CI** | **Authorship** and merge intent; gates before history is shared. |
| **ReleasePanel Central** | **Deployment intent** — what should run where (desired state, enrollment, receipts). |
| **VPS / runtime** | **Bounded pull and apply** — consume an observed ref; reload services; no silent publishing. |
| **Agent** | **Runtime observation** — heartbeat, inventory, health; convergence reporting when enabled. |

## Read-only deploy keys (policy as machinery)

GitHub **deploy keys on the server stay read-only by design**.

That is not only least-privilege security; it is **operational doctrine**:

- **Source authority** stays with GitHub (and humans/CI that merge there).
- **Runtime authority** stays bounded to apply + observe on the node.
- **Deployment causality** stays linear: intent flows through GitHub → pull → observed commit → apply → report.

Without that constraint, runtimes tend toward **mutable snowflakes**: hot edits, mystery commits, and “works only on prod” drift.

## Explainable deploy semantics

The story operators should be able to tell without hidden steps:

1. **Deploy requested** (intent recorded via normal release path).
2. **Server fetches** the repo from GitHub.
3. **Runtime resets** (or checks out) to the **observed** branch/commit.
4. **Reload** (services, agent cycle, etc.) runs against that tree.
5. **Heartbeat / reports** reflect **observed** commit and runtime state.

Every stage is **inspectable** (SSH, git, logs, configs), **attributable** (SHA maps to GitHub), and **operationally legible**.

## What we avoid

- Hot edits on the server as the norm.
- Push-from-production or mutable git histories driven by the VPS.
- Deploy divergence without a matching GitHub narrative.
- Hidden runtime mutation that breaks reproducibility.

This aims at **immutable-ish runtime semantics** relative to a declared ref **without** orchestrator complexity: operators retain familiar SSH and file inspection while the platform keeps **convergence discipline**.

## Related documents

- [Architecture overview](ARCHITECTURE.md)
- [Central HTTP API](CENTRAL_API.md)
- [Filesystem layout](FILESYSTEM.md)
