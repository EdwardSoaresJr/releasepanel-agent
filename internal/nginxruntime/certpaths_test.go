package nginxruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateReadableCertPair_ok(t *testing.T) {
	root := t.TempDir()
	cert := filepath.Join(root, "cert.pem")
	key := filepath.Join(root, "key.pem")
	if err := os.WriteFile(cert, []byte("CERT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReadableCertPair(cert, key, []string{root}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReadableCertPair_rejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "other.pem")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReadableCertPair(outside, outside, []string{root}); err == nil {
		t.Fatal("expected error")
	}
}
