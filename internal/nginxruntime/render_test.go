package nginxruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWebRoot_laravelPublic(t *testing.T) {
	r, err := ResolveWebRoot("/srv/foo/repo", "laravel")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/srv/foo/repo", "public")
	if r != want {
		t.Fatalf("got %q want %q", r, want)
	}
}

func TestValidateDomain_rejectsInjection(t *testing.T) {
	if ValidateDomain("ex ample.com") == nil {
		t.Fatal("expected error")
	}
	if ValidateDomain("evil.com;") == nil {
		t.Fatal("expected error")
	}
}

func TestRenderSiteConfig_static(t *testing.T) {
	body, err := RenderSiteConfig(RenderInput{
		SiteULID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Domain:         "example.com",
		WebRoot:        "/srv/sites/example/public",
		RuntimeType:    "static",
		PHPFastcgiPass: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body == "" {
		t.Fatal("empty body")
	}
	if testing.Verbose() {
		t.Log(body)
	}
}

func TestRenderSiteConfig_laravelRequiresFastcgi(t *testing.T) {
	_, err := RenderSiteConfig(RenderInput{
		SiteULID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Domain:         "example.com",
		WebRoot:        "/srv/sites/example/public",
		RuntimeType:    "laravel",
		PHPFastcgiPass: "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderSiteConfig_tlsRedirect(t *testing.T) {
	body, err := RenderSiteConfig(RenderInput{
		SiteULID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Domain:         "example.com",
		WebRoot:        "/srv/sites/example/public",
		RuntimeType:    "static",
		PHPFastcgiPass: "",
		TLS: &TLSMaterialization{
			CertificatePath:     "/etc/ssl/example/fullchain.pem",
			CertificateKeyPath:  "/etc/ssl/example/privkey.pem",
			RedirectHTTPToHTTPS: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "listen 443 ssl") || !strings.Contains(body, "return 301 https://") {
		t.Fatalf("expected TLS redirect server blocks, got:\n%s", body)
	}
}

func TestRenderSiteConfig_tlsHttpAndHttpsCoexist(t *testing.T) {
	body, err := RenderSiteConfig(RenderInput{
		SiteULID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Domain:         "example.com",
		WebRoot:        "/srv/sites/example/public",
		RuntimeType:    "static",
		PHPFastcgiPass: "",
		TLS: &TLSMaterialization{
			CertificatePath:    "/etc/ssl/example/fullchain.pem",
			CertificateKeyPath: "/etc/ssl/example/privkey.pem",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	c80 := strings.Count(body, "listen 80")
	if c80 < 2 {
		t.Fatalf("expected HTTP app + HTTPS listeners, got listen 80 count=%d", c80)
	}
	if !strings.Contains(body, "listen 443 ssl") {
		t.Fatal("missing HTTPS listener")
	}
}
