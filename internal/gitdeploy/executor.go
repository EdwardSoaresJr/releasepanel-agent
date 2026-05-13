package gitdeploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"releasepanel/agent/pkg/api"
)

// Repository deploy observation states (Central SiteRepositoryDeployState).
const (
	StateFetching = "fetching"
	StateFetched  = "fetched"
	StateApplying = "applying"
	StateApplied  = "applied"
	StateFailed   = "failed"
)

// Reporter posts one site_runtime_reports row (ping receipt).
type Reporter func(row api.SiteRuntimeReportRow) error

// Options drives bounded clone/fetch/reset for one intent.
type Options struct {
	Intent           api.SiteRepositoryDeployIntent
	AllowedPathRoots []string
	PrivateKeyPath   string
	KnownHostsPath   string
	TLSStateEcho     string
}

// CommitObservation is runtime evidence after reset (not a deployment narrative).
type CommitObservation struct {
	SHA       string
	Subject   string
	Author    string
	ObservedAt time.Time
}

// Run executes clone-or-fetch, reset --hard, and emits Central state transitions via reporter.
func Run(ctx context.Context, reporter Reporter, opts Options) error {
	if reporter == nil {
		return fmt.Errorf("reporter nil")
	}
	tls := strings.TrimSpace(opts.TLSStateEcho)
	if tls == "" {
		tls = "none"
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return reporter(failedRow(opts.Intent.SiteULID, tls, "git executable not found"))
	}

	intent := opts.Intent
	deployPath := strings.TrimSpace(intent.DeployPath)
	branch := strings.TrimSpace(intent.Branch)
	gitURL := strings.TrimSpace(intent.GitSSHURL)

	if branch == "" || deployPath == "" || gitURL == "" {
		return reporter(failedRow(intent.SiteULID, tls, "intent missing branch, deploy_path, or git_ssh_url"))
	}

	if err := ValidateDeployPath(deployPath, opts.AllowedPathRoots); err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "invalid deploy path: "+err.Error()))
	}

	if !intent.RepositoryDeployKeyConfigured {
		return reporter(failedRow(intent.SiteULID, tls, "repository deploy key not configured on site ledger"))
	}

	keyPath := strings.TrimSpace(opts.PrivateKeyPath)
	if keyPath == "" {
		return reporter(failedRow(intent.SiteULID, tls, "deploy private key path empty"))
	}
	st, err := os.Stat(keyPath)
	if err != nil || !st.Mode().IsRegular() {
		return reporter(failedRow(intent.SiteULID, tls, "deploy private key missing or not a regular file"))
	}
	if st.Mode().Perm()&0o077 != 0 {
		return reporter(failedRow(intent.SiteULID, tls, "deploy private key permissions too permissive (expected not group/world readable)"))
	}

	sshCmd, err := GitSSHCommand(keyPath, opts.KnownHostsPath)
	if err != nil {
		return reporter(failedRow(intent.SiteULID, tls, "ssh command: "+err.Error()))
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_SSH_COMMAND="+sshCmd)

	site := intent.SiteULID
	cur := strings.TrimSpace(strings.ToLower(intent.DeployState))

	skipFetchingReceipt := cur == StateFetched || cur == StateApplying
	if !skipFetchingReceipt {
		if err := reporter(row(site, tls, StateFetching, "", nil)); err != nil {
			return err
		}
	}

	parent := filepath.Dir(deployPath)
	if err := ValidateDeployPath(parent, opts.AllowedPathRoots); err != nil {
		return reporter(failedRow(site, tls, "invalid deploy path parent: "+err.Error()))
	}
	pst, err := os.Stat(parent)
	if err != nil || !pst.IsDir() {
		return reporter(failedRow(site, tls, "deploy path parent directory missing"))
	}

	repoExists, err := gitRepoExists(deployPath)
	if err != nil {
		return reporter(failedRow(site, tls, err.Error()))
	}

	if !repoExists {
		_, statErr := os.Stat(deployPath)
		if statErr == nil {
			return reporter(failedRow(site, tls, "deploy path exists but is not a git repository"))
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return reporter(failedRow(site, tls, "deploy path stat failed"))
		}

		if _, err := runGit(ctx, gitPath, "", env, "clone", "--branch", branch, "--", gitURL, deployPath); err != nil {
			return reporter(failedRow(site, tls, boundedReason("site deploy clone failed", err)))
		}
	} else {
		if _, err := runGit(ctx, gitPath, deployPath, env, "fetch", "origin"); err != nil {
			return reporter(failedRow(site, tls, boundedReason("site deploy fetch failed", err)))
		}
	}

	skipFetchedReceipt := cur == StateApplying
	if !skipFetchedReceipt {
		if err := reporter(row(site, tls, StateFetched, "", nil)); err != nil {
			return err
		}
	}
	if err := reporter(row(site, tls, StateApplying, "", nil)); err != nil {
		return err
	}

	resetTarget := "origin/" + branch
	if sha := strings.TrimSpace(intent.DesiredCommitSHA); sha != "" {
		if err := gitVerifyCommit(ctx, gitPath, deployPath, env, sha); err != nil {
			return reporter(failedRow(site, tls, boundedReason("desired commit not found after fetch", err)))
		}
		resetTarget = sha
	}

	if _, err := runGit(ctx, gitPath, deployPath, env, "reset", "--hard", resetTarget); err != nil {
		return reporter(failedRow(site, tls, boundedReason("site deploy reset failed", err)))
	}

	commit, err := readCommitObservation(ctx, gitPath, deployPath, env)
	if err != nil {
		return reporter(failedRow(site, tls, boundedReason("read observed commit failed", err)))
	}

	return reporter(appliedRow(site, tls, commit))
}

func gitRepoExists(dir string) (bool, error) {
	st, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !st.IsDir() {
		return false, fmt.Errorf("deploy path is not a directory")
	}
	gitDir := filepath.Join(dir, ".git")
	gst, err := os.Stat(gitDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if gst.IsDir() || gst.Mode().IsRegular() {
		return true, nil
	}
	return false, fmt.Errorf("deploy path .git invalid")
}

func runGit(ctx context.Context, gitPath, workDir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gitPath, args...)
	cmd.Env = env
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%w: %s", err, truncateRunes(msg, 4000))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitVerifyCommit(ctx context.Context, gitPath, workDir string, env []string, sha string) error {
	_, err := runGit(ctx, gitPath, workDir, env, "cat-file", "-e", sha+"^{commit}")
	return err
}

func readCommitObservation(ctx context.Context, gitPath, workDir string, env []string) (*CommitObservation, error) {
	shaOut, err := runGit(ctx, gitPath, workDir, env, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	raw, err := runGit(ctx, gitPath, workDir, env, "log", "-1", "-z", "--format=%H%x00%s%x00%an <%ae>")
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "\x00")
	parts := strings.Split(raw, "\x00")
	if len(parts) == 4 && parts[3] == "" {
		parts = parts[:3]
	}
	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected git log format")
	}
	now := time.Now().UTC()
	return &CommitObservation{
		SHA:        strings.TrimSpace(parts[0]),
		Subject:    truncateRunes(strings.TrimSpace(parts[1]), 9000),
		Author:     truncateRunes(strings.TrimSpace(parts[2]), 500),
		ObservedAt: now,
	}, nil
}

func row(siteULID, tls, repoState, fail string, commit *CommitObservation) api.SiteRuntimeReportRow {
	r := api.SiteRuntimeReportRow{
		SiteULID:                      siteULID,
		TLSState:                      tls,
		ObservedAt:                    api.RFC3339UTC(time.Now().UTC()),
		RepositoryDeployState:         repoState,
		RepositoryDeployFailureReason: fail,
	}
	if commit != nil {
		r.ObservedCommitSHA = commit.SHA
		r.ObservedCommitMessage = commit.Subject
		r.ObservedCommitAuthor = commit.Author
		r.ObservedCommitObservedAt = api.RFC3339UTC(commit.ObservedAt)
	}
	return r
}

func failedRow(siteULID, tls, reason string) api.SiteRuntimeReportRow {
	return api.SiteRuntimeReportRow{
		SiteULID:                      siteULID,
		TLSState:                      tls,
		ObservedAt:                    api.RFC3339UTC(time.Now().UTC()),
		RepositoryDeployState:         StateFailed,
		RepositoryDeployFailureReason: truncateRunes(reason, 19000),
	}
}

func appliedRow(siteULID, tls string, c *CommitObservation) api.SiteRuntimeReportRow {
	return row(siteULID, tls, StateApplied, "", c)
}

func boundedReason(prefix string, err error) string {
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
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
