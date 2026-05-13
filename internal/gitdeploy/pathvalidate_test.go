package gitdeploy

import "testing"

func TestValidateDeployPath(t *testing.T) {
	roots := []string{"/srv/releasepanel", "/var/www"}

	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"/srv/releasepanel/foo", true},
		{"/srv/releasepanel", true},
		{"/var/www/x/y", true},
		{"", false},
		{"relative/path", false},
		{"/tmp/outside", false},
		{"/", false},
		{"/srv/releasepanel/../etc/passwd", false},
	} {
		err := ValidateDeployPath(tc.path, roots)
		if tc.ok && err != nil {
			t.Fatalf("%q: want ok got %v", tc.path, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: want error", tc.path)
		}
	}
}
