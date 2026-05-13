package nginxruntime

import (
	"fmt"
	"strings"

	"releasepanel/agent/pkg/api"
)

// TLSMaterialization is ledger-driven TLS termination materialization (paths issued outside the agent).
type TLSMaterialization struct {
	CertificatePath     string
	CertificateKeyPath  string
	RedirectHTTPToHTTPS bool
}

// TLSMaterializationFromIntent returns non-nil when the ping hint requests TLS blocks.
func TLSMaterializationFromIntent(in api.SiteRuntimeApplyIntent) (*TLSMaterialization, error) {
	if !in.TLSEnabled {
		return nil, nil
	}
	c := strings.TrimSpace(in.SSLCertificatePath)
	k := strings.TrimSpace(in.SSLCertificateKeyPath)
	if c == "" || k == "" {
		return nil, fmt.Errorf("tls enabled but certificate paths missing on intent")
	}
	return &TLSMaterialization{
		CertificatePath:     c,
		CertificateKeyPath:  k,
		RedirectHTTPToHTTPS: in.RedirectHTTPToHTTPS,
	}, nil
}

// TLSObservationAfterReload sets tls_state on the reload receipt when the ledger is ready for APPLIED.
func TLSObservationAfterReload(intent api.SiteRuntimeApplyIntent) string {
	if !intent.TLSEnabled {
		return ""
	}
	ls := strings.ToLower(strings.TrimSpace(intent.TlsLedgerState))
	if ls == "issued" {
		return "applied"
	}
	return ""
}
