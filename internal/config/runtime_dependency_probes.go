package config

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"releasepanel/agent/internal/runtimeprobe"
)

const maxRuntimeDependencyProbes = 32

// RuntimeDependencyProbe is explicit YAML configuration only (no discovery).
type RuntimeDependencyProbe struct {
	Type          string   `yaml:"type"`
	Socket        string   `yaml:"socket"`
	TCP           string   `yaml:"tcp"`
	SystemctlArgv []string `yaml:"systemctl_argv"`
	PidFile       string   `yaml:"pid_file"`
}

var allowedRuntimeDependencyTypes = map[string]struct{}{
	"php_fpm":  {},
	"redis":    {},
	"mysql":    {},
	"postgres": {},
}

func compileRuntimeDependencyProbes(raw []RuntimeDependencyProbe) ([]runtimeprobe.Spec, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > maxRuntimeDependencyProbes {
		return nil, fmt.Errorf("runtime_dependency_probes: at most %d entries", maxRuntimeDependencyProbes)
	}
	out := make([]runtimeprobe.Spec, 0, len(raw))
	for i, r := range raw {
		dep := strings.ToLower(strings.TrimSpace(r.Type))
		if dep == "" {
			return nil, fmt.Errorf("runtime_dependency_probes[%d]: type is required", i)
		}
		if _, ok := allowedRuntimeDependencyTypes[dep]; !ok {
			return nil, fmt.Errorf("runtime_dependency_probes[%d]: unsupported type %q (allowed: php_fpm, redis, mysql, postgres)", i, dep)
		}
		socket := strings.TrimSpace(r.Socket)
		tcp := strings.TrimSpace(r.TCP)
		switch dep {
		case "php_fpm":
			if socket == "" {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: php_fpm requires socket", i)
			}
			if tcp != "" {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: php_fpm must not set tcp", i)
			}
			if !filepath.IsAbs(socket) {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: socket must be absolute path", i)
			}
		case "redis", "mysql", "postgres":
			if tcp == "" {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: %s requires tcp host:port", i, dep)
			}
			if socket != "" {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: %s must not set socket", i, dep)
			}
			host, port, err := net.SplitHostPort(tcp)
			if err != nil || host == "" || port == "" {
				return nil, fmt.Errorf("runtime_dependency_probes[%d]: tcp must be host:port", i)
			}
		}
		pf := strings.TrimSpace(r.PidFile)
		if pf != "" && !filepath.IsAbs(pf) {
			return nil, fmt.Errorf("runtime_dependency_probes[%d]: pid_file must be absolute path", i)
		}
		argv, err := validateSystemctlArgv(r.SystemctlArgv, i)
		if err != nil {
			return nil, err
		}
		out = append(out, runtimeprobe.Spec{
			Dependency:    dep,
			SocketPath:    socket,
			TCPAddr:       tcp,
			SystemctlArgv: argv,
			PidFile:       pf,
		})
	}
	return out, nil
}

func validateSystemctlArgv(argv []string, idx int) ([]string, error) {
	if len(argv) == 0 {
		return nil, nil
	}
	const maxArgs = 24
	if len(argv) > maxArgs {
		return nil, fmt.Errorf("runtime_dependency_probes[%d]: systemctl_argv has at most %d elements", idx, maxArgs)
	}
	out := make([]string, 0, len(argv))
	for j, a := range argv {
		a = strings.TrimSpace(a)
		if a == "" {
			return nil, fmt.Errorf("runtime_dependency_probes[%d]: systemctl_argv[%d] empty", idx, j)
		}
		if len(a) > 1024 {
			return nil, fmt.Errorf("runtime_dependency_probes[%d]: systemctl_argv[%d] too long", idx, j)
		}
		out = append(out, a)
	}
	if !filepath.IsAbs(out[0]) {
		return nil, fmt.Errorf("runtime_dependency_probes[%d]: systemctl_argv[0] must be absolute executable path", idx)
	}
	return out, nil
}
