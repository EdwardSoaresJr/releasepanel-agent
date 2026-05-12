package nginx

import (
	"bytes"
	"os/exec"
	"strings"
	"time"

	"releasepanel/agent/pkg/api"
)

// Inventory discovers nginx binary metadata without requiring root.
func Inventory() *api.NginxInventory {
	path, err := exec.LookPath("nginx")
	if err != nil || path == "" {
		return nil
	}
	cmd := exec.Command(path, "-v")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	_ = cmd.Run()
	return &api.NginxInventory{
		BinaryPath: path,
		Version:    strings.TrimSpace(stderr.String()),
	}
}

// ConfigTest runs `nginx -t` and reports stdout/stderr combined.
func ConfigTest() (ok bool, detail string, elapsed time.Duration) {
	start := time.Now()
	path, err := exec.LookPath("nginx")
	if err != nil {
		return false, "nginx binary not found", time.Since(start)
	}
	cmd := exec.Command(path, "-t")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return err == nil, strings.TrimSpace(buf.String()), time.Since(start)
}

// Reload executes an explicit reload sequence suitable for appliances.
// Order: `nginx -t` then `nginx -s reload`. Both must succeed for Reload to return nil.
func Reload() error {
	path, err := exec.LookPath("nginx")
	if err != nil {
		return err
	}
	test := exec.Command(path, "-t")
	var testOut bytes.Buffer
	test.Stdout = &testOut
	test.Stderr = &testOut
	if err := test.Run(); err != nil {
		return err
	}
	rel := exec.Command(path, "-s", "reload")
	var relOut bytes.Buffer
	rel.Stdout = &relOut
	rel.Stderr = &relOut
	if err := rel.Run(); err != nil {
		return err
	}
	return nil
}
