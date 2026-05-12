package config

import (
	"fmt"
	"os"
	"time"

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
}

// PollInterval returns the configured reconcile interval or the default.
func (c *Config) PollInterval() time.Duration {
	if c.PollIntervalSeconds <= 0 {
		return DefaultPollInterval
	}
	return time.Duration(c.PollIntervalSeconds) * time.Second
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

	return &cfg, nil
}
