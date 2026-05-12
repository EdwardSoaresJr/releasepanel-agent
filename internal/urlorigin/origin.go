// Package urlorigin validates central_base_url values: scheme + host (+ port), never a path prefix.
package urlorigin

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateHTTPOrigin rejects URLs that include a path or query (e.g. https://host/api breaks routing).
func ValidateHTTPOrigin(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return fmt.Errorf("parse central URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("central_base_url must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("central_base_url must include a host")
	}
	if strings.Trim(u.Path, "/") != "" {
		return fmt.Errorf("central_base_url must be origin-only (no path); got path %q — use https://example.com and route /api/v1 on Central (see docs/CENTRAL_API.md)", u.Path)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("central_base_url must not include query or fragment")
	}
	return nil
}
