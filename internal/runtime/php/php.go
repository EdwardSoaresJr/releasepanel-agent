package php

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"time"

	"releasepanel/agent/pkg/api"
)

// Inventory discovers PHP CLI metadata when present.
func Inventory() *api.PHPInventory {
	path, err := exec.LookPath("php")
	if err != nil || path == "" {
		return nil
	}
	cmd := exec.Command(path, "-v")
	out, err := cmd.Output()
	if err != nil {
		return &api.PHPInventory{CLIPath: path, FPM: defaultFPMHint()}
	}
	first := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return &api.PHPInventory{
		CLIPath:    path,
		CLIVersion: first,
		FPM:        defaultFPMHint(),
	}
}

func defaultFPMHint() string {
	return "php-fpm"
}

// CLIProbe verifies `php -v` executes successfully.
func CLIProbe() (ok bool, detail string, elapsed time.Duration) {
	start := time.Now()
	path, err := exec.LookPath("php")
	if err != nil {
		return false, "php binary not found", time.Since(start)
	}
	cmd := exec.Command(path, "-v")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return err == nil, strings.TrimSpace(buf.String()), time.Since(start)
}

// FPM_SOCKET_HINTS are checked for existence only (no HTTP fastcgi in v1).
var FPM_SOCKET_HINTS = []string{
	"/run/php/php-fpm.sock",
	"/run/php/php8.2-fpm.sock",
	"/run/php/php8.3-fpm.sock",
	"/var/run/php/php-fpm.sock",
}

// FPMListenProbe reports whether a unix socket hint exists.
func FPMListenProbe() (ok bool, detail string, elapsed time.Duration) {
	start := time.Now()
	for _, p := range FPM_SOCKET_HINTS {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return true, "socket " + p, time.Since(start)
		}
	}
	return false, "no known php-fpm unix socket found", time.Since(start)
}
