package health

import (
	"time"

	"releasepanel/agent/internal/runtime/nginx"
	"releasepanel/agent/internal/runtime/php"
	"releasepanel/agent/pkg/api"
)

// Collect executes bounded probes suitable for frequent scheduling.
func Collect(nodeID string) api.HealthReport {
	now := time.Now().UTC().Format(time.RFC3339)
	checks := make([]api.HealthCheck, 0, 4)

	if ok, detail, d := nginx.ConfigTest(); ok {
		checks = append(checks, api.HealthCheck{Name: "nginx_config_test", OK: true, Detail: detail, DurationMs: d.Milliseconds()})
	} else {
		checks = append(checks, api.HealthCheck{Name: "nginx_config_test", OK: false, Detail: detail, DurationMs: d.Milliseconds()})
	}

	if ok, detail, d := php.CLIProbe(); ok {
		checks = append(checks, api.HealthCheck{Name: "php_cli", OK: true, Detail: detail, DurationMs: d.Milliseconds()})
	} else {
		checks = append(checks, api.HealthCheck{Name: "php_cli", OK: false, Detail: detail, DurationMs: d.Milliseconds()})
	}

	if ok, detail, d := php.FPMListenProbe(); ok {
		checks = append(checks, api.HealthCheck{Name: "php_fpm_socket", OK: true, Detail: detail, DurationMs: d.Milliseconds()})
	} else {
		checks = append(checks, api.HealthCheck{Name: "php_fpm_socket", OK: false, Detail: detail, DurationMs: d.Milliseconds()})
	}

	return api.HealthReport{
		SchemaVersion: api.SchemaV1,
		NodeID:        nodeID,
		CollectedAt:   now,
		Checks:        checks,
	}
}
