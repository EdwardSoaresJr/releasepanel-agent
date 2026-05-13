package nginxruntime

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveWebRoot maps ledger deploy_path + runtime_type to nginx root (deterministic).
func ResolveWebRoot(deployPath, runtimeType string) (string, error) {
	dp := strings.TrimSpace(deployPath)
	if dp == "" {
		return "", fmt.Errorf("empty deploy_path")
	}
	rt := strings.ToLower(strings.TrimSpace(runtimeType))
	if rt == "" {
		rt = "static"
	}
	switch rt {
	case "laravel":
		return filepath.Join(dp, "public"), nil
	case "wordpress", "wp":
		return dp, nil
	case "php":
		return dp, nil
	case "static":
		return dp, nil
	case "custom":
		return dp, nil
	default:
		return "", fmt.Errorf("unsupported runtime_type %q", runtimeType)
	}
}

// ValidateDomain rejects characters unsafe inside nginx server_name / injection.
func ValidateDomain(d string) error {
	d = strings.TrimSpace(d)
	if d == "" {
		return fmt.Errorf("empty domain")
	}
	if len(d) > 253 {
		return fmt.Errorf("domain too long")
	}
	for _, ch := range d {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '.' || ch == '-':
		default:
			return fmt.Errorf("invalid domain character")
		}
	}
	return nil
}

// ValidateLinuxPath rejects obvious nginx directive injection in filesystem paths.
func ValidateLinuxPath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("path must be absolute")
	}
	if strings.ContainsAny(p, ";\n\r`\"'$\\") {
		return fmt.Errorf("invalid path characters")
	}
	return nil
}
