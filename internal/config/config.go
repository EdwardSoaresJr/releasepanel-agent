package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"releasepanel/agent/internal/urlorigin"
	"releasepanel/agent/internal/runtimeprobe"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath   = "/etc/releasepanel-agent/config.yaml"
	DefaultStateDir     = "/var/lib/releasepanel-agent"
	DefaultLogDir       = "/var/log/releasepanel-agent"
	DefaultPollInterval = 30 * time.Second
)

// Config is static appliance configuration loaded at startup.
type Config struct {
	CentralBaseURL string `yaml:"central_base_url"`
	// PollIntervalSeconds is the agent main-loop sleep between reconcile cycles.
	PollIntervalSeconds int `yaml:"poll_interval_seconds"`

	StateDir string `yaml:"state_dir"`
	LogDir   string `yaml:"log_dir"`

	SkipTLSVerify bool `yaml:"skip_tls_verify"`

	// ManifestReconcileEnabled runs GET desired + deploy pipeline each cycle when true.
	// Default false: heartbeat/inventory/health only until Central is ready — see docs/CENTRAL_API.md.
	ManifestReconcileEnabled bool `yaml:"manifest_reconcile_enabled"`

	// AgentPingTokenFile is a filesystem path to the Sanctum agent bearer token (e.g. from POST /api/v1/agent/bootstrap).
	// When non-empty, the agent observes site_repository_deploy_intents via GET /api/v1/agent/ping and posts repository receipts on POST ping.
	AgentPingTokenFile string `yaml:"agent_ping_token_file"`

	// RuntimeDeployPathRoots lists absolute path prefixes allowed for site deploy_path (repository convergence). Required when agent_ping_token_file is set.
	RuntimeDeployPathRoots []string `yaml:"runtime_deploy_path_roots"`

	// GitHubSSHKnownHostsFile optionally pins ssh.github.com/github.com host keys (StrictHostKeyChecking=yes). When empty, StrictHostKeyChecking=accept-new is used (bounded bootstrap).
	GitHubSSHKnownHostsFile string `yaml:"github_ssh_known_hosts_file"`

	// NginxSitesAvailableRoot is an absolute directory for agent-written site configs (e.g. /etc/nginx/sites-available/releasepanel).
	NginxSitesAvailableRoot string `yaml:"nginx_sites_available_root"`
	// NginxSitesEnabledRoot is an absolute directory for symlinks included by nginx (e.g. /etc/nginx/sites-enabled/releasepanel).
	NginxSitesEnabledRoot string `yaml:"nginx_sites_enabled_root"`
	// NginxTestArgv is argv for config validation (default: /usr/sbin/nginx -t).
	NginxTestArgv []string `yaml:"nginx_test_argv"`
	// NginxReloadArgv is argv for reload (default: /usr/sbin/nginx -s reload).
	NginxReloadArgv []string `yaml:"nginx_reload_argv"`
	// NginxPHPFastcgiPass is the fastcgi_pass target for PHP-backed runtimes (e.g. unix:/run/php/php8.3-fpm.sock).
	NginxPHPFastcgiPass string `yaml:"nginx_php_fastcgi_pass"`

	// TLSCertificatePathRoots lists absolute path prefixes allowed for ssl_certificate_path / ssl_certificate_key_path when tls_enabled (e.g. /etc/letsencrypt).
	TLSCertificatePathRoots []string `yaml:"tls_certificate_path_roots"`

	// RuntimeDependencyProbes lists explicit bounded probes (unix socket, tcp, optional systemctl argv, optional pid file).
	RuntimeDependencyProbes []RuntimeDependencyProbe `yaml:"runtime_dependency_probes"`

	probeSpecs []runtimeprobe.Spec `yaml:"-"`
}

// RuntimeProbeSpecs returns compiled probes after Load (nil or empty if unset).
func (c *Config) RuntimeProbeSpecs() []runtimeprobe.Spec {
	if len(c.probeSpecs) == 0 {
		return nil
	}
	return c.probeSpecs
}

// PollInterval returns the configured reconcile interval or the default.
func (c *Config) PollInterval() time.Duration {
	if c.PollIntervalSeconds <= 0 {
		return DefaultPollInterval
	}
	return time.Duration(c.PollIntervalSeconds) * time.Second
}

// NginxRuntimeConfigured returns true when bounded nginx materialization paths are set.
func (c *Config) NginxRuntimeConfigured() bool {
	return strings.TrimSpace(c.NginxSitesAvailableRoot) != "" && strings.TrimSpace(c.NginxSitesEnabledRoot) != ""
}

// ResolvedNginxTestArgv returns explicit argv for nginx -t (bounded defaults).
func (c *Config) ResolvedNginxTestArgv() []string {
	if len(c.NginxTestArgv) > 0 {
		return append([]string(nil), c.NginxTestArgv...)
	}
	return []string{"/usr/sbin/nginx", "-t"}
}

// ResolvedNginxReloadArgv returns explicit argv for nginx reload (bounded defaults).
func (c *Config) ResolvedNginxReloadArgv() []string {
	if len(c.NginxReloadArgv) > 0 {
		return append([]string(nil), c.NginxReloadArgv...)
	}
	return []string{"/usr/sbin/nginx", "-s", "reload"}
}

// ResolvedNginxPHPFastcgiPass returns fastcgi_pass target with an appliance default.
func (c *Config) ResolvedNginxPHPFastcgiPass() string {
	if s := strings.TrimSpace(c.NginxPHPFastcgiPass); s != "" {
		return s
	}
	return "unix:/run/php/php8.3-fpm.sock"
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("RELEASEPANEL_CONFIG")
	}
	if path == "" {
		path = DefaultConfigPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.CentralBaseURL == "" {
		return nil, fmt.Errorf("central_base_url is required")
	}
	if err := urlorigin.ValidateHTTPOrigin(cfg.CentralBaseURL); err != nil {
		return nil, fmt.Errorf("central_base_url: %w", err)
	}

	if cfg.StateDir == "" {
		cfg.StateDir = os.Getenv("RELEASEPANEL_STATE_DIR")
	}
	if cfg.StateDir == "" {
		cfg.StateDir = DefaultStateDir
	}

	if cfg.LogDir == "" {
		cfg.LogDir = os.Getenv("RELEASEPANEL_LOG_DIR")
	}
	if cfg.LogDir == "" {
		cfg.LogDir = DefaultLogDir
	}

	if strings.TrimSpace(cfg.AgentPingTokenFile) != "" && len(cfg.RuntimeDeployPathRoots) == 0 {
		return nil, fmt.Errorf("runtime_deploy_path_roots is required when agent_ping_token_file is set")
	}

	ar := strings.TrimSpace(cfg.NginxSitesAvailableRoot)
	er := strings.TrimSpace(cfg.NginxSitesEnabledRoot)
	if (ar != "") != (er != "") {
		return nil, fmt.Errorf("nginx_sites_available_root and nginx_sites_enabled_root must both be set or both empty")
	}
	if ar != "" {
		if !filepath.IsAbs(ar) || !filepath.IsAbs(er) {
			return nil, fmt.Errorf("nginx sites roots must be absolute paths")
		}
		if filepath.Clean(ar) == string(filepath.Separator) || filepath.Clean(er) == string(filepath.Separator) {
			return nil, fmt.Errorf("nginx sites roots reject filesystem root")
		}
	}

	specs, err := compileRuntimeDependencyProbes(cfg.RuntimeDependencyProbes)
	if err != nil {
		return nil, err
	}
	cfg.probeSpecs = specs

	return &cfg, nil
}
