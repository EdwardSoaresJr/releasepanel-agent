//go:build unix

package inventory

import (
	"syscall"
)

func statFS(path string) (total, free uint64, err error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return 0, 0, err
	}
	total = uint64(s.Blocks) * uint64(s.Bsize)
	free = uint64(s.Bfree) * uint64(s.Bsize)
	return total, free, nil
}
