# Enrollment & Authentication

## Goal

Establish a **cryptographically bounded** trust relationship between one VPS node and **releasepanel-central**, using an operator-supplied enrollment secret, then persist **node-only** credentials locally.

## Preconditions

- Agent binary installed and config pointing at `central_base_url` (HTTPS).
- Operator has an enrollment token from central (opaque string; format is central’s concern).

## Flow

1. **Operator** runs enrollment (or cloud-init writes token file and invokes enroll):

   ```text
   releasepanel-agent enroll \
     --central-url https://central.example \
     --token-file /run/releasepanel/enrollment.token
   ```

2. **Agent** reads token, collects **stable node facts** (hostname, OS, architecture, `/etc/machine-id` when present), builds `POST /v1/nodes/enroll` body (see `pkg/api`).

3. **Central** validates token, allocates `node_id`, returns:

   - `node_id` (string)
   - `api_key` or bearer secret for subsequent calls (v1: opaque secret)
   - Optional: `endpoints` overrides (advanced; default paths relative to base URL)

4. **Agent** writes **`enrollment.json`** atomically:

   - `node_id`, `api_key`, `central_base_url`, `enrolled_at` (RFC3339 UTC)

5. **Agent** sets file mode **0600** and owner root (when run as root on appliance).

## Subsequent requests

All authenticated calls send:

```http
Authorization: Bearer <api_key>
X-Releasepanel-Node-Id: <node_id>
```

(Exact header names are fixed in `pkg/api`; central must accept the same.)

## Rotation & re-enrollment

- **v1**: Re-running `enroll` with a valid central-issued replacement token **replaces** `enrollment.json` after explicit operator action.
- Central may revoke keys; agent surfaces `401`/`403` in logs and exits non-zero from `run` after backoff (implementation detail in agent loop).

## Local-first note

Central does not need to trust DNS beyond TLS server name; the node persists identity locally. Deleting `enrollment.json` requires re-enrollment.
