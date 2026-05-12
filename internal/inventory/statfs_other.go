//go:build !unix

package inventory

import "fmt"

func statFS(path string) (total, free uint64, err error) {
	return 0, 0, fmt.Errorf("disk inventory not implemented on this platform")
}
