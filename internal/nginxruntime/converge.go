package nginxruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"releasepanel/agent/internal/gitdeploy"
	"releasepanel/agent/pkg/api"
)

// Bounded runtime_apply_state observations (Central SiteRuntimeApplyState).
const (
	StateRequested       = "requested"
	StateApplying        = "applying"
	StateApplied         = "applied"
	StateReloadRequested = "reload_requested"
	StateReloadApplied   = "reload_applied"
	StateFailed          = "failed"
)

// Reporter posts one site_runtime_reports row (ping receipt).
type Reporter func(row api.SiteRuntimeReportRow) error

// Options configures bounded nginx materialization + reload convergence.
type Options struct {
	Intent api.SiteRuntimeApplyIntent

	DeployPathRoots []string // same bounded roots as git/deploy_path ledger

	NginxSitesAvailableRoot string
	NginxSitesEnabledRoot   string
	NginxTestArgv           []string
	NginxReloadArgv         []string
	NginxPHPFastcgiPass     string

	TLSStateEcho string

	// TLSCertificatePathRoots bounds TLS PEM paths on disk (e.g. /etc/letsencrypt). Required when intent tls_enabled.
	TLSCertificatePathRoots []string
}

// Run performs apply leg (requested/applying) or reload leg (reload_requested).
func Run(ctx context.Context, reporter Reporter, opts Options) error {
	if reporter == nil {
		return fmt.Errorf("reporter nil")
	}
	tls := strings.TrimSpace(opts.TLSStateEcho)
	if tls == "" {
		tls = "none"
	}

	cur := strings.TrimSpace(strings.ToLower(opts.Intent.RuntimeApplyState))
	switch cur {
	case StateRequested, StateApplying:
		return runApplyLeg(ctx, reporter, opts, tls, cur)
	case StateReloadRequested:
		return runReloadLeg(ctx, reporter, opts, tls)
	default:
		return nil
	}
}

func runApplyLeg(ctx context.Context, reporter Reporter, opts Options, tls, cur string) error {
	intent := opts.Intent
	deployPath := strings.TrimSpace(intent.DeployPath)
	if err := gitdeploy.ValidateDeployPath(deployPath, opts.DeployPathRoots); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "invalid deploy_path: "+err.Error()))
	}

	webRoot, err := ResolveWebRoot(deployPath, intent.RuntimeType)
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, err.Error()))
	}
	if err := gitdeploy.ValidateDeployPath(webRoot, opts.DeployPathRoots); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "invalid web root: "+err.Error()))
	}

	tlsMat, err := validateTLSMaterialization(reporter, intent, opts, tls)
	if err != nil {
		return err
	}

	if err := prepareNginxDirs(reporter, intent.SiteULID, tls, opts.NginxSitesAvailableRoot, opts.NginxSitesEnabledRoot); err != nil {
		return err
	}

	skipApplyingReceipt := cur == StateApplying
	if !skipApplyingReceipt {
		if err := reporter(rowRuntime(intent.SiteULID, tls, StateApplying, "")); err != nil {
			return err
		}
	}

	conf, err := RenderSiteConfig(RenderInput{
		SiteULID:        intent.SiteULID,
		Domain:          intent.Domain,
		WebRoot:         webRoot,
		RuntimeType:     intent.RuntimeType,
		PHPFastcgiPass:  opts.NginxPHPFastcgiPass,
		TLS:             tlsMat,
	})
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("nginx config render failed", err)))
	}

	availPath, enabledPath, err := siteConfigPaths(opts.NginxSitesAvailableRoot, opts.NginxSitesEnabledRoot, intent.SiteULID)
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, err.Error()))
	}

	if err := WriteAtomicFile(availPath, conf); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("write nginx site config failed", err)))
	}

	if err := EnsureSymlink(enabledPath, availPath); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("symlink nginx enabled site failed", err)))
	}

	if _, err := runArgv(ctx, opts.NginxTestArgv); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("nginx validation failed", err)))
	}

	return reporter(rowRuntime(intent.SiteULID, tls, StateApplied, ""))
}

func runReloadLeg(ctx context.Context, reporter Reporter, opts Options, tls string) error {
	intent := opts.Intent
	deployPath := strings.TrimSpace(intent.DeployPath)
	if err := gitdeploy.ValidateDeployPath(deployPath, opts.DeployPathRoots); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "invalid deploy_path: "+err.Error()))
	}

	webRoot, err := ResolveWebRoot(deployPath, intent.RuntimeType)
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, err.Error()))
	}
	if err := gitdeploy.ValidateDeployPath(webRoot, opts.DeployPathRoots); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "invalid web root: "+err.Error()))
	}

	tlsMat, err := validateTLSMaterialization(reporter, intent, opts, tls)
	if err != nil {
		return err
	}

	if err := prepareNginxDirs(reporter, intent.SiteULID, tls, opts.NginxSitesAvailableRoot, opts.NginxSitesEnabledRoot); err != nil {
		return err
	}

	conf, err := RenderSiteConfig(RenderInput{
		SiteULID:       intent.SiteULID,
		Domain:         intent.Domain,
		WebRoot:        webRoot,
		RuntimeType:    intent.RuntimeType,
		PHPFastcgiPass: opts.NginxPHPFastcgiPass,
		TLS:            tlsMat,
	})
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("nginx config render failed", err)))
	}

	availPath, enabledPath, err := siteConfigPaths(opts.NginxSitesAvailableRoot, opts.NginxSitesEnabledRoot, intent.SiteULID)
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, err.Error()))
	}

	if err := WriteAtomicFile(availPath, conf); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("write nginx site config failed", err)))
	}

	if err := EnsureSymlink(enabledPath, availPath); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("symlink nginx enabled site failed", err)))
	}

	if _, err := runArgv(ctx, opts.NginxTestArgv); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("nginx validation failed", err)))
	}

	if _, err := runArgv(ctx, opts.NginxReloadArgv); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, boundedFail("nginx reload failed", err)))
	}

	reloadAt := time.Now().UTC()
	tlsOut := tls
	if adv := TLSObservationAfterReload(intent); adv != "" {
		tlsOut = adv
	}
	return reporter(rowReloadApplied(intent.SiteULID, tlsOut, reloadAt))
}

func rowRuntime(siteULID, tls, applyState, fail string) api.SiteRuntimeReportRow {
	return api.SiteRuntimeReportRow{
		SiteULID:          siteULID,
		TLSState:          tls,
		ObservedAt:        api.RFC3339UTC(time.Now().UTC()),
		RuntimeApplyState: applyState,
		FailureReason:     fail,
	}
}

func rowReloadApplied(siteULID, tls string, reloadAt time.Time) api.SiteRuntimeReportRow {
	ts := api.RFC3339UTC(reloadAt)
	return api.SiteRuntimeReportRow{
		SiteULID:                siteULID,
		TLSState:                tls,
		ObservedAt:              api.RFC3339UTC(time.Now().UTC()),
		RuntimeApplyState:       StateReloadApplied,
		RuntimeReloadObservedAt: ts,
	}
}

func failedRow(siteULID, tls, reason string) api.SiteRuntimeReportRow {
	return api.SiteRuntimeReportRow{
		SiteULID:          siteULID,
		TLSState:          tls,
		ObservedAt:        api.RFC3339UTC(time.Now().UTC()),
		RuntimeApplyState: StateFailed,
		FailureReason:     truncateRunes(reason, 19000),
	}
}

func validateTLSMaterialization(reporter Reporter, intent api.SiteRuntimeApplyIntent, opts Options, tlsEcho string) (*TLSMaterialization, error) {
	tlsMat, err := TLSMaterializationFromIntent(intent)
	if err != nil {
		_ = reporter(failedRow(intent.SiteULID, tlsEcho, err.Error()))
		return nil, err
	}
	if tlsMat == nil {
		return nil, nil
	}
	if len(opts.TLSCertificatePathRoots) == 0 {
		reason := "tls_certificate_path_roots not configured"
		_ = reporter(failedRow(intent.SiteULID, tlsEcho, reason))
		return nil, fmt.Errorf("%s", reason)
	}
	if err := ValidateReadableCertPair(tlsMat.CertificatePath, tlsMat.CertificateKeyPath, opts.TLSCertificatePathRoots); err != nil {
		reason := boundedFail("certificate path validation failed", err)
		_ = reporter(failedRow(intent.SiteULID, tlsEcho, reason))
		return nil, fmt.Errorf("certificate path validation failed")
	}
	return tlsMat, nil
}

func prepareNginxDirs(reporter Reporter, siteULID, tls, avail, enabled string) error {
	a := strings.TrimSpace(avail)
	e := strings.TrimSpace(enabled)
	if a == "" || e == "" {
		reason := "nginx sites roots not configured"
		_ = reporter(failedRow(siteULID, tls, reason))
		return fmt.Errorf("%s", reason)
	}
	a = filepath.Clean(a)
	e = filepath.Clean(e)
	if err := os.MkdirAll(a, 0o755); err != nil {
		reason := boundedFail("mkdir nginx sites-available root failed", err)
		_ = reporter(failedRow(siteULID, tls, reason))
		return fmt.Errorf("%s", reason)
	}
	if err := os.MkdirAll(e, 0o755); err != nil {
		reason := boundedFail("mkdir nginx sites-enabled root failed", err)
		_ = reporter(failedRow(siteULID, tls, reason))
		return fmt.Errorf("%s", reason)
	}
	return nil
}

func boundedFail(prefix string, err error) string {
	if err == nil {
		return prefix
	}
	return truncateRunes(prefix+": "+err.Error(), 19000)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func runArgv(ctx context.Context, argv []string) (string, error) {
	if len(argv) < 1 || strings.TrimSpace(argv[0]) == "" {
		return "", fmt.Errorf("nginx argv missing executable")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		if out != "" {
			return out, fmt.Errorf("%w: %s", err, truncateRunes(out, 4000))
		}
		return out, err
	}
	return out, nil
}

func siteConfigPaths(availableRoot, enabledRoot, siteULID string) (availablePath, enabledPath string, err error) {
	ar := strings.TrimSpace(availableRoot)
	er := strings.TrimSpace(enabledRoot)
	if ar == "" || er == "" {
		return "", "", fmt.Errorf("nginx sites roots not configured")
	}
	id := strings.TrimSpace(siteULID)
	if id == "" {
		return "", "", fmt.Errorf("empty site_ulid")
	}
	name := fmt.Sprintf("rp-%s.conf", strings.ToLower(id))
	if strings.Contains(name, "..") || strings.Contains(name, string(filepath.Separator)) {
		return "", "", fmt.Errorf("invalid site_ulid")
	}
	ap := filepath.Join(ar, name)
	ep := filepath.Join(er, name)
	if err := pathUnderRoot(ap, ar); err != nil {
		return "", "", err
	}
	if err := pathUnderRoot(ep, er); err != nil {
		return "", "", err
	}
	return ap, ep, nil
}

func pathUnderRoot(p, root string) error {
	pc := filepath.Clean(p)
	rc := filepath.Clean(root)
	if pc == rc || strings.HasPrefix(pc, rc+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("path outside nginx root")
}

// WriteAtomicFile writes content to path atomically (same directory).
func WriteAtomicFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".rp-nginx-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// EnsureSymlink sets linkpath as a symlink to target, replacing a stale symlink only.
func EnsureSymlink(linkpath, target string) error {
	fi, err := os.Lstat(linkpath)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			cur, err := os.Readlink(linkpath)
			if err != nil {
				return err
			}
			if cur == target {
				return nil
			}
			if err := os.Remove(linkpath); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("nginx enabled path exists and is not a symlink")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, linkpath)
}
