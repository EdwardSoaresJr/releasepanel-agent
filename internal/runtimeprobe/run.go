package runtimeprobe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"releasepanel/agent/pkg/api"
)

const (
	defaultDialTimeout = 400 * time.Millisecond
	maxProbeDuration   = 2 * time.Second
)

// RunAll executes each spec with a bounded deadline. Order matches specs.
func RunAll(ctx context.Context, specs []Spec) []api.RuntimeDependencyReportRow {
	if len(specs) == 0 {
		return nil
	}
	out := make([]api.RuntimeDependencyReportRow, 0, len(specs))
	for _, sp := range specs {
		pctx, cancel := context.WithTimeout(ctx, maxProbeDuration)
		row := runOne(pctx, sp)
		cancel()
		out = append(out, row)
	}
	return out
}

func runOne(ctx context.Context, sp Spec) api.RuntimeDependencyReportRow {
	dep := strings.TrimSpace(sp.Dependency)
	if dep == "" {
		return failRow(dep, "empty dependency name")
	}
	if sp.SocketPath != "" && sp.TCPAddr != "" {
		return failRow(dep, "probe declares both socket and tcp")
	}

	hasPrimary := sp.SocketPath != "" || sp.TCPAddr != ""
	hasSecondary := len(sp.SystemctlArgv) > 0 || strings.TrimSpace(sp.PidFile) != ""
	if !hasPrimary && !hasSecondary {
		return failRow(dep, "probe has no checks configured")
	}

	var failureReason string
	st := StateObserved

	if sp.SocketPath != "" {
		sockSt, sockReason := probeUnixSocket(ctx, sp.SocketPath)
		if sockSt != StateObserved {
			st = sockSt
			failureReason = sockReason
		}
	} else if sp.TCPAddr != "" {
		tcpSt, tcpReason := probeTCP(ctx, sp.TCPAddr)
		if tcpSt != StateObserved {
			st = tcpSt
			failureReason = tcpReason
		}
	}

	if st == StateObserved && len(sp.SystemctlArgv) > 0 {
		uSt, uReason := probeSystemctl(ctx, sp.SystemctlArgv)
		if uSt != StateObserved {
			st = uSt
			failureReason = uReason
		}
	}

	if st == StateObserved && strings.TrimSpace(sp.PidFile) != "" {
		pSt, pReason := probePidFile(ctx, strings.TrimSpace(sp.PidFile))
		if pSt != StateObserved {
			st = pSt
			failureReason = pReason
		}
	}

	obs := api.RFC3339UTC(time.Now().UTC())
	if st == StateObserved {
		return api.RuntimeDependencyReportRow{
			Dependency: dep,
			State:      StateObserved,
			ObservedAt: obs,
		}
	}
	return api.RuntimeDependencyReportRow{
		Dependency:    dep,
		State:         st,
		ObservedAt:    obs,
		FailureReason: failureReason,
	}
}

func failRow(dep, reason string) api.RuntimeDependencyReportRow {
	return api.RuntimeDependencyReportRow{
		Dependency:    dep,
		State:         StateFailed,
		ObservedAt:    api.RFC3339UTC(time.Now().UTC()),
		FailureReason: reason,
	}
}

func probeUnixSocket(ctx context.Context, path string) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return StateFailed, "socket path not absolute"
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StateMissing, "socket path missing"
		}
		return StateFailed, fmt.Sprintf("socket stat: %v", err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return StateFailed, "path exists but is not a socket"
	}

	d := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return StateFailed, fmt.Sprintf("socket dial: %v", err)
		}
		if ctx.Err() != nil {
			return StateUnreachable, "socket dial timed out"
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return StateUnreachable, "socket dial timed out"
		}
		// Refused or other: dependency present but not accepting connections.
		return StateUnreachable, fmt.Sprintf("socket dial: %v", err)
	}
	_ = conn.Close()
	return StateObserved, ""
}

func probeTCP(ctx context.Context, addr string) (string, string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return StateFailed, "empty tcp address"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || port == "" {
		return StateFailed, "tcp address must be host:port"
	}

	d := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		if ctx.Err() != nil {
			return StateUnreachable, "tcp connect timed out"
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return StateUnreachable, "tcp connect timed out"
		}
		return StateUnreachable, fmt.Sprintf("tcp connect: %v", err)
	}
	_ = conn.Close()
	return StateObserved, ""
}

func probeSystemctl(ctx context.Context, argv []string) (string, string) {
	if len(argv) < 2 {
		return StateFailed, "systemctl_argv requires at least executable and one argument"
	}
	bin := strings.TrimSpace(argv[0])
	if bin == "" {
		return StateFailed, "systemctl_argv executable empty"
	}

	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Env = minimalProcEnv()
	out, err := cmd.CombinedOutput()
	exit := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	} else if err != nil {
		return StateFailed, fmt.Sprintf("systemctl exec: %v", err)
	}
	if exit != 0 {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "non-zero exit"
		}
		const maxCtlOut = 2000
		if len(msg) > maxCtlOut {
			msg = msg[:maxCtlOut] + "…"
		}
		return StateMissing, msg
	}
	return StateObserved, ""
}

func probePidFile(ctx context.Context, path string) (string, string) {
	if !filepath.IsAbs(path) {
		return StateFailed, "pid_file must be absolute"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StateMissing, "pid file missing"
		}
		return StateFailed, fmt.Sprintf("pid file read: %v", err)
	}
	pidStr := strings.TrimSpace(string(raw))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return StateFailed, "pid file did not contain a positive integer"
	}

	select {
	case <-ctx.Done():
		return StateUnreachable, "pid probe timed out"
	default:
	}

	ok, aliveErr := pidAlive(pid)
	if aliveErr != nil {
		return StateFailed, fmt.Sprintf("pid check: %v", aliveErr)
	}
	if !ok {
		return StateMissing, "pid not running"
	}
	return StateObserved, ""
}

func minimalProcEnv() []string {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			return []string{e}
		}
	}
	return nil
}
