package inventory

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"releasepanel/agent/internal/runtime/nginx"
	"releasepanel/agent/internal/runtime/php"
	"releasepanel/agent/pkg/api"
)

func machineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return strings.TrimSpace(string(b))
	}
	return ""
}

func Collect(hostname, agentVersion string) (api.InventoryReport, error) {
	h := hostname
	if h == "" {
		h, _ = os.Hostname()
	}

	total, free, err := statFS("/")
	var disk []api.DiskMount
	if err == nil {
		disk = append(disk, api.DiskMount{
			Path:       "/",
			FSType:     "",
			TotalBytes: total,
			FreeBytes:  free,
		})
	}

	ngx := nginx.Inventory()
	ph := php.Inventory()

	report := api.InventoryReport{
		SchemaVersion: api.SchemaV1,
		CollectedAt:   time.Now().UTC().Format(time.RFC3339),
		Facts: api.NodeFacts{
			Hostname:     h,
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			MachineID:    machineID(),
			AgentVersion: agentVersion,
		},
		Runtimes: api.RuntimeInventory{
			Nginx: ngx,
			PHP:   ph,
		},
		Disk: disk,
	}
	return report, nil
}

func NginxBinaryVersion(binaryPath string) (*api.NginxInventory, error) {
	if binaryPath == "" {
		binaryPath = lookupPath("nginx")
		if binaryPath == "" {
			return nil, nil
		}
	}
	cmd := exec.Command(binaryPath, "-v")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		return &api.NginxInventory{BinaryPath: binaryPath}, nil
	}
	v := strings.TrimSpace(stderr.String())
	return &api.NginxInventory{BinaryPath: binaryPath, Version: v}, nil
}

func PHPCLIVersion(cliPath string) (*api.PHPInventory, error) {
	if cliPath == "" {
		cliPath = lookupPath("php")
		if cliPath == "" {
			return nil, nil
		}
	}
	cmd := exec.Command(cliPath, "-v")
	out, err := cmd.Output()
	if err != nil {
		return &api.PHPInventory{CLIPath: cliPath}, nil
	}
	first := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return &api.PHPInventory{CLIPath: cliPath, CLIVersion: first, FPM: "php-fpm"}, nil
}

func lookupPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return p
}
