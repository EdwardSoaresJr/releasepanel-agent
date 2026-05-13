package gitdeploy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GitSSHCommand builds GIT_SSH_COMMAND for a bounded read-only deploy key (no global SSH config).
func GitSSHCommand(privateKeyPath string, knownHostsFile string) (string, error) {
	key := strings.TrimSpace(privateKeyPath)
	if key == "" {
		return "", fmt.Errorf("empty deploy key path")
	}
	if !filepath.IsAbs(key) {
		return "", fmt.Errorf("deploy key path must be absolute")
	}
	key = filepath.Clean(key)

	var b strings.Builder
	b.WriteString("ssh -i ")
	b.WriteString(shellSingleQuote(key))
	b.WriteString(" -o IdentitiesOnly=yes")
	kh := strings.TrimSpace(knownHostsFile)
	if kh != "" {
		if !filepath.IsAbs(kh) {
			return "", fmt.Errorf("known_hosts path must be absolute")
		}
		kh = filepath.Clean(kh)
		b.WriteString(" -o StrictHostKeyChecking=yes -o UserKnownHostsFile=")
		b.WriteString(shellSingleQuote(kh))
	} else {
		b.WriteString(" -o StrictHostKeyChecking=accept-new")
	}
	return b.String(), nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
