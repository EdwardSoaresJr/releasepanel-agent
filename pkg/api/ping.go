package api

import "time"

// AgentPingResponse is the JSON body returned by GET/POST /api/v1/agent/ping (subset used by the agent).
type AgentPingResponse struct {
	SiteRepositoryDeployIntents   []SiteRepositoryDeployIntent   `json:"site_repository_deploy_intents"`
	SiteRuntimeApplyIntents       []SiteRuntimeApplyIntent       `json:"site_runtime_apply_intents"`
}

// SiteRepositoryDeployIntent mirrors Central SiteRepositoryDeployPingHints intent rows.
type SiteRepositoryDeployIntent struct {
	SiteULID                      string `json:"site_ulid"`
	DeployPath                    string `json:"deploy_path"`
	GitSSHURL                     string `json:"git_ssh_url"`
	Branch                        string `json:"branch"`
	DesiredCommitSHA              string `json:"desired_commit_sha,omitempty"`
	DeployState                   string `json:"deploy_state"`
	RepositoryDeployKeyConfigured bool   `json:"repository_deploy_key_configured"`
}

// SiteRuntimeApplyIntent mirrors Central SiteRuntimeApplyPingHints intent rows.
type SiteRuntimeApplyIntent struct {
	SiteULID              string `json:"site_ulid"`
	Domain                string `json:"domain"`
	DeployPath            string `json:"deploy_path"`
	RuntimeType           string `json:"runtime_type"`
	RuntimeApplyState     string `json:"runtime_apply_state"`
	TLSEnabled            bool   `json:"tls_enabled"`
	SSLCertificatePath    string `json:"ssl_certificate_path,omitempty"`
	SSLCertificateKeyPath string `json:"ssl_certificate_key_path,omitempty"`
	RedirectHTTPToHTTPS   bool   `json:"redirect_http_to_https"`
	TlsLedgerState        string `json:"tls_ledger_state,omitempty"`
}

// SiteRuntimeReportRow is one element of POST /api/v1/agent/ping site_runtime_reports.
// tls_state and observed_at are required by Central validation whenever this row is sent.
type SiteRuntimeReportRow struct {
	SiteULID                       string `json:"site_ulid"`
	TLSState                       string `json:"tls_state"`
	ObservedAt                     string `json:"observed_at"`
	RepositoryDeployState          string `json:"repository_deploy_state,omitempty"`
	RepositoryDeployFailureReason  string `json:"repository_deploy_failure_reason,omitempty"`
	ObservedCommitSHA              string `json:"observed_commit_sha,omitempty"`
	ObservedCommitMessage          string `json:"observed_commit_message,omitempty"`
	ObservedCommitAuthor           string `json:"observed_commit_author,omitempty"`
	ObservedCommitObservedAt       string `json:"observed_commit_observed_at,omitempty"`
	RuntimeApplyState              string `json:"runtime_apply_state,omitempty"`
	RuntimeReloadObservedAt        string `json:"runtime_reload_observed_at,omitempty"`
	// FailureReason is shared by TLS and runtime apply convergence on Central (PingController passes it to SiteRuntimeApplyConvergence).
	FailureReason                  string `json:"failure_reason,omitempty"`
}

// AgentPingPostBody is the outbound POST body for /api/v1/agent/ping (minimal fields for receipts).
type AgentPingPostBody struct {
	SchemaVersion      int                    `json:"schema_version,omitempty"`
	Hostname           string                 `json:"hostname,omitempty"`
	AgentVersion       string                 `json:"agent_version,omitempty"`
	SiteRuntimeReports []SiteRuntimeReportRow `json:"site_runtime_reports,omitempty"`

	// RuntimeDependencyReports are bounded dependency observations only (not synthetic health).
	// Each row uses convergence vocabulary state: observed | missing | unreachable | failed.
	RuntimeDependencyReports []RuntimeDependencyReportRow `json:"runtime_dependency_reports,omitempty"`
}

// RuntimeDependencyReportRow is one runtime dependency observation on POST /api/v1/agent/ping.
type RuntimeDependencyReportRow struct {
	Dependency    string `json:"dependency"`
	State         string `json:"state"`
	ObservedAt    string `json:"observed_at"`
	FailureReason string `json:"failure_reason,omitempty"`
}

// RFC3339UTC formats t in UTC with Z suffix for Central date validation.
func RFC3339UTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
