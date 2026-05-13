package gitdeploy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateDeployPath ensures p is a non-empty absolute path under one of allowedRoots with no traversal escape.
// allowedRoots entries must be absolute, non-empty, and cleaned; deploy path must be strictly scoped (no "/" alone).
func ValidateDeployPath(p string, allowedRoots []string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("deploy path empty")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("deploy path must be absolute")
	}
	clean := filepath.Clean(p)
	if clean == "/" || clean == "." {
		return fmt.Errorf("deploy path rejects root")
	}
	if strings.Contains(clean, "..") {
		return fmt.Errorf("deploy path traversal rejected")
	}

	if len(allowedRoots) == 0 {
		return fmt.Errorf("no runtime deploy path roots configured")
	}

	for i := range allowedRoots {
		r := strings.TrimSpace(allowedRoots[i])
		if r == "" {
			continue
		}
		if !filepath.IsAbs(r) {
			return fmt.Errorf("runtime deploy path root must be absolute")
		}
		rootClean := filepath.Clean(r)
		if rootClean == "/" {
			return fmt.Errorf("runtime deploy path root rejects filesystem root")
		}
		if clean == rootClean || strings.HasPrefix(clean, rootClean+string(filepath.Separator)) {
			return nil
		}
	}

	return fmt.Errorf("deploy path outside allowed runtime roots")
}
