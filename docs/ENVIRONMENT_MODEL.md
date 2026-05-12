# Environment & Application Model (Local)

This agent treats **environment** and **application** as central-declared concepts materialized on disk through deployment manifests. On the node, we only persist **fingerprints** and **staging outputs**; the authoritative naming lives in central.

## Terms

| Term | Meaning |
|------|---------|
| **Environment** | A deployment slice for one logical stage (`production`, `staging`, …). Central assigns manifests scoped by environment. |
| **Application** | A deployable unit (repo artifact + runtime bindings). Central enumerates applications per environment. |
| **Manifest / desired state** | Versioned JSON describing what must be true on the node for one reconcile pass (fingerprints, artifact refs, ordered steps — expanded later). |
| **Runtime bindings** | nginx vhosts, php pools, systemd units — **declared** by manifest steps and executed by the deploy runner; adapters remain thin. |

## Local representation (v1)

On disk:

- `convergence.json` records **`desired_fingerprint`** vs **`applied_fingerprint`** for the **currently enrolled node**.
- `deploy/staging/<run_id>/` receives extracted artifacts when prepares steps exist.
- nginx/php binaries and sockets are **discovered** for inventory/health; config ownership transitions happen only through future APPLY steps.

There is **no** secondary registry SQLite/db on the node — inspecting JSON files is sufficient for support.

## Determinism

Central computes **`fingerprint`** over canonical serialized manifest bytes. The agent never guesses fingerprints; it applies exactly what central declares per fetch cycle.

## Evolution

When manifests gain explicit `environment` and `application_id` fields, inventory reports may echo those keys read-only from the last applied manifest cache — still central-authoritative.
