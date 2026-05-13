//go:build unix

package runtimeprobe

import (
	"errors"
	"syscall"
)

// pidAlive reports whether pid exists (signal 0). False + nil error means known absent.
func pidAlive(pid int) (ok bool, err error) {
	e := syscall.Kill(pid, syscall.Signal(0))
	if e == nil {
		return true, nil
	}
	if errors.Is(e, syscall.ESRCH) {
		return false, nil
	}
	return false, e
}
