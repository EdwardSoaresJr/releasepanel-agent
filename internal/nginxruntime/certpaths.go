package nginxruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"releasepanel/agent/internal/gitdeploy"
)

// ValidateReadableCertPair ensures PEM paths are absolute, bounded by allowed roots, non-traversal, and readable regular files.
func ValidateReadableCertPair(certPath, keyPath string, allowedRoots []string) error {
	certPath = strings.TrimSpace(certPath)
	keyPath = strings.TrimSpace(keyPath)
	if certPath == "" || keyPath == "" {
		return fmt.Errorf("certificate or key path empty")
	}
	if err := validateReadableCertFile(certPath, allowedRoots); err != nil {
		return fmt.Errorf("certificate path: %w", err)
	}
	if err := validateReadableCertFile(keyPath, allowedRoots); err != nil {
		return fmt.Errorf("certificate key path: %w", err)
	}
	return nil
}

func validateReadableCertFile(p string, allowedRoots []string) error {
	if err := ValidateLinuxPath(p); err != nil {
		return err
	}
	clean := filepath.Clean(p)
	resolved := clean
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		resolved = filepath.Clean(r)
	}
	if len(allowedRoots) > 0 {
		if err := gitdeploy.ValidateDeployPath(resolved, allowedRoots); err != nil {
			return fmt.Errorf("outside tls_certificate_path_roots: %w", err)
		}
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("not accessible: %w", err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	f, err := os.Open(resolved)
	if err != nil {
		return fmt.Errorf("not readable: %w", err)
	}
	_ = f.Close()
	return nil
}
