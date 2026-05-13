package nginxruntime

import (
	"fmt"
	"strings"
)

// RenderInput is the bounded identity passed into the fixed nginx template.
type RenderInput struct {
	SiteULID       string
	Domain         string
	WebRoot        string
	RuntimeType    string
	PHPFastcgiPass string
	TLS            *TLSMaterialization
}

// RenderSiteConfig returns deterministic nginx config text for one site (no external templates).
func RenderSiteConfig(in RenderInput) (string, error) {
	if err := ValidateDomain(in.Domain); err != nil {
		return "", err
	}
	if err := ValidateLinuxPath(in.WebRoot); err != nil {
		return "", err
	}
	ulid := strings.TrimSpace(in.SiteULID)
	if ulid == "" {
		return "", fmt.Errorf("empty site_ulid")
	}

	rt := strings.ToLower(strings.TrimSpace(in.RuntimeType))
	if rt == "" {
		rt = "static"
	}
	if rt == "wp" {
		rt = "wordpress"
	}

	locationTry, phpBlock, err := buildPHPLocations(rt, in.PHPFastcgiPass)
	if err != nil {
		return "", err
	}

	header := fmt.Sprintf("# ReleasePanel bounded nginx site — site_ulid=%s — agent-managed\n", ulid)

	if in.TLS != nil {
		body, err := renderTLSNginx(in.Domain, in.WebRoot, in.TLS, locationTry, phpBlock)
		if err != nil {
			return "", err
		}
		return header + body, nil
	}

	body := renderHTTPServer(in.Domain, in.WebRoot, locationTry, phpBlock)
	return header + body, nil
}

func buildPHPLocations(rt, phpPass string) (locationTry string, phpBlock string, err error) {
	locationTry = tryFilesForRuntime(rt)
	includePHP := rt == "laravel" || rt == "wordpress" || rt == "php" || rt == "custom"

	pass := strings.TrimSpace(phpPass)
	if includePHP && pass == "" {
		return "", "", fmt.Errorf("nginx_php_fastcgi_pass required for runtime_type %q", rt)
	}
	if err := validateFastcgiPass(pass); err != nil {
		return "", "", err
	}

	var b strings.Builder
	if includePHP {
		b.WriteString("\n    location ~ \\.php$ {\n")
		b.WriteString("        include fastcgi_params;\n")
		b.WriteString("        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\n")
		b.WriteString(fmt.Sprintf("        fastcgi_pass %s;\n", pass))
		b.WriteString("    }\n")
	}
	return locationTry, b.String(), nil
}

func appCore(webRoot, locationTry, phpBlock string) string {
	return fmt.Sprintf(`    root %s;
    index index.html index.php;

    location / {
        %s
    }%s`, webRoot, locationTry, phpBlock)
}

func renderHTTPServer(domain, webRoot, locationTry, phpBlock string) string {
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
%s
}
`, domain, appCore(webRoot, locationTry, phpBlock))
}

func renderTLSNginx(domain, webRoot string, tls *TLSMaterialization, locationTry, phpBlock string) (string, error) {
	if tls == nil {
		return "", fmt.Errorf("tls materialization nil")
	}
	cert := strings.TrimSpace(tls.CertificatePath)
	key := strings.TrimSpace(tls.CertificateKeyPath)
	if cert == "" || key == "" {
		return "", fmt.Errorf("tls certificate paths empty")
	}
	if err := ValidateLinuxPath(cert); err != nil {
		return "", fmt.Errorf("certificate path: %w", err)
	}
	if err := ValidateLinuxPath(key); err != nil {
		return "", fmt.Errorf("certificate key path: %w", err)
	}

	core := appCore(webRoot, locationTry, phpBlock)

	https := fmt.Sprintf(`server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name %s;
    ssl_certificate %s;
    ssl_certificate_key %s;
%s
}
`, domain, cert, key, core)

	if tls.RedirectHTTPToHTTPS {
		httpRedirect := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
    return 301 https://$host$request_uri;
}
`, domain)
		return httpRedirect + https, nil
	}

	httpApp := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
%s
}
`, domain, core)

	return httpApp + https, nil
}

func tryFilesForRuntime(rt string) string {
	switch rt {
	case "laravel", "php":
		return "try_files $uri $uri/ /index.php?$query_string;"
	case "wordpress":
		return "try_files $uri $uri/ /index.php?$args;"
	case "static":
		return "try_files $uri $uri/ =404;"
	case "custom":
		return "try_files $uri $uri/ /index.php?$query_string;"
	default:
		return "try_files $uri $uri/ =404;"
	}
}

func validateFastcgiPass(pass string) error {
	if pass == "" {
		return nil
	}
	if strings.ContainsAny(pass, ";\n\r`\"'\\") {
		return fmt.Errorf("invalid fastcgi_pass value")
	}
	return nil
}
