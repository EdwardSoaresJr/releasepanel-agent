//go:build !unix

package runtimeprobe

import "fmt"

func pidAlive(pid int) (bool, error) {
	return false, fmt.Errorf("pid_file probe unsupported on this platform")
}
