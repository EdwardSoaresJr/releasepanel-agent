package lock

import (
	"errors"
	"fmt"
	"os"
)

// ErrHeld indicates another process holds the lock file.
var ErrHeld = errors.New("lock already held")

// ExclusiveCreate acquires an exclusive lock file by atomic create.
// The caller must Close or rely on process exit to release; deleting the file releases explicitly.
func ExclusiveCreate(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrHeld, path)
		}
		return nil, err
	}
	return f, nil
}

func Release(f *os.File, path string) {
	if f != nil {
		_ = f.Close()
	}
	if path != "" {
		_ = os.Remove(path)
	}
}
