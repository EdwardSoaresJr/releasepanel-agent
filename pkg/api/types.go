// Package api defines stable JSON contracts between the node agent and releasepanel-central.
// Schema versions are bumped only when fields are removed or semantics change.
package api

const (
	SchemaV1 = 1
)

// EnrollRequest is sent from agent to central during enrollment.
type EnrollRequest struct {
	SchemaVersion int               `json:"schema_version"`
	Token         string            `json:"token"`
	NodeFacts     NodeFacts         `json:"node_facts"`
}

// EnrollResponse is returned by central after successful enrollment.
type EnrollResponse struct {
	SchemaVersion int    `json:"schema_version"`
	NodeID        string `json:"node_id"`
	APIKey        string `json:"api_key"`
}

// NodeFacts are collected locally and treated as opaque identifiers by central.
type NodeFacts struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	MachineID    string `json:"machine_id,omitempty"`
	AgentVersion string `json:"agent_version"`
}

// InventoryReport is the slow-changing snapshot pushed to central.
type InventoryReport struct {
	SchemaVersion int                    `json:"schema_version"`
	NodeID        string                 `json:"node_id"`
	CollectedAt   string                 `json:"collected_at"`
	Facts         NodeFacts              `json:"facts"`
	Runtimes      RuntimeInventory       `json:"runtimes"`
	Disk          []DiskMount            `json:"disk"`
}

type RuntimeInventory struct {
	Nginx *NginxInventory `json:"nginx,omitempty"`
	PHP   *PHPInventory   `json:"php,omitempty"`
}

type NginxInventory struct {
	BinaryPath string `json:"binary_path,omitempty"`
	Version    string `json:"version,omitempty"`
}

type PHPInventory struct {
	CLIPath    string `json:"cli_path,omitempty"`
	CLIVersion string `json:"cli_version,omitempty"`
	FPM        string `json:"fpm_service_hint,omitempty"`
}

type DiskMount struct {
	Path         string `json:"path"`
	FSType       string `json:"fs_type,omitempty"`
	TotalBytes   uint64 `json:"total_bytes,omitempty"`
	FreeBytes    uint64 `json:"free_bytes,omitempty"`
}

// HealthReport is the fast-changing probe summary pushed to central.
type HealthReport struct {
	SchemaVersion int           `json:"schema_version"`
	NodeID        string        `json:"node_id"`
	CollectedAt   string        `json:"collected_at"`
	Checks        []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name       string `json:"name"`
	OK         bool   `json:"ok"`
	Detail     string `json:"detail,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// HeartbeatReport lets Central record agent presence (e.g. last_seen_at).
type HeartbeatReport struct {
	SchemaVersion int    `json:"schema_version"`
	NodeID        string `json:"node_id"`
	CollectedAt   string `json:"collected_at"`
	AgentVersion  string `json:"agent_version"`
}

// ConvergenceReport ties desired/applied fingerprints to deploy outcomes.
type ConvergenceReport struct {
	SchemaVersion           int    `json:"schema_version"`
	NodeID                  string `json:"node_id"`
	CollectedAt             string `json:"collected_at"`
	DesiredFingerprint      string `json:"desired_fingerprint,omitempty"`
	AppliedFingerprint      string `json:"applied_fingerprint,omitempty"`
	LastReportedFingerprint string `json:"last_reported_fingerprint,omitempty"`
	LastDeployRunID         string `json:"last_deploy_run_id,omitempty"`
	PendingConvergence      bool   `json:"pending_convergence"`
}

// DesiredManifest is fetched from central; body is intentionally minimal for scaffolding.
type DesiredManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Fingerprint   string `json:"fingerprint"`
	Raw           []byte `json:"-"`
}
