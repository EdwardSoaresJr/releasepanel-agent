package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"releasepanel/agent/internal/central"
	"releasepanel/agent/internal/lock"
	"releasepanel/agent/internal/paths"
	"releasepanel/agent/internal/state"
	"releasepanel/agent/pkg/api"
)

type Phase string

const (
	PhaseFetchDesired Phase = "FETCH_DESIRED"
	PhaseValidate     Phase = "VALIDATE"
	PhasePrepare      Phase = "PREPARE"
	PhaseApply        Phase = "APPLY"
	PhaseVerify       Phase = "VERIFY"
	PhaseReport       Phase = "REPORT"
)

type RunRecord struct {
	RunID     string    `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
	Phase     Phase     `json:"phase"`
	Message   string    `json:"message,omitempty"`
}

// Runner wires the explicit pipeline; Apply remains a no-op beyond scaffolding hooks.
type Runner struct {
	StateDir string
}

func fingerprintFromPayload(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var stub struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(raw, &stub); err != nil {
		return ""
	}
	return stub.Fingerprint
}

// Reconcile executes one bounded attempt toward convergence when desired differs from applied.
func (r *Runner) Reconcile(ctx context.Context, cl *central.Client, conv *state.ConvergenceRecord, nodeID string) (*state.ConvergenceRecord, error) {
	lockPath := filepath.Join(paths.LocksDir(r.StateDir), "deploy.lock")
	if err := os.MkdirAll(paths.LocksDir(r.StateDir), 0o755); err != nil {
		return conv, err
	}

	f, err := lock.ExclusiveCreate(lockPath)
	if err != nil {
		if errors.Is(err, lock.ErrHeld) {
			return conv, nil
		}
		return conv, err
	}
	defer lock.Release(f, lockPath)

	runID := time.Now().UTC().Format("20060102T150405Z")
	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseFetchDesired, Message: "fetch desired manifest"})
	raw, fp, err := cl.FetchDesired(ctx)
	if err != nil {
		_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseFetchDesired, Message: err.Error()})
		return conv, err
	}
	if len(raw) == 0 && fp == "" {
		_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseFetchDesired, Message: "no desired manifest"})
		return conv, nil
	}
	if fp == "" {
		fp = fingerprintFromPayload(raw)
	}

	conv.DesiredFingerprint = fp
	if fp != "" && conv.AppliedFingerprint == fp && conv.LastReportedFingerprint == fp {
		return conv, nil
	}

	conv.LastDeployRunID = runID
	conv.UpdatedAt = time.Now().UTC()

	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseValidate, Message: "noop validate"})
	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhasePrepare, Message: "ensure staging dir"})
	if err := os.MkdirAll(filepath.Join(paths.DeployStaging(r.StateDir), runID), 0o755); err != nil {
		return conv, err
	}

	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseApply, Message: "noop apply (scaffold)"})

	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseVerify, Message: "noop verify (scaffold)"})

	// Scaffold marks applied == desired without mutating system state.
	conv.AppliedFingerprint = conv.DesiredFingerprint
	conv.UpdatedAt = time.Now().UTC()
	conv.SchemaVersion = api.SchemaV1
	if err := state.WriteJSONAtomic(paths.ConvergenceState(r.StateDir), conv, 0o644); err != nil {
		return conv, err
	}

	report := api.ConvergenceReport{
		SchemaVersion:           api.SchemaV1,
		NodeID:                  nodeID,
		CollectedAt:             time.Now().UTC().Format(time.RFC3339),
		DesiredFingerprint:      conv.DesiredFingerprint,
		AppliedFingerprint:      conv.AppliedFingerprint,
		LastReportedFingerprint: conv.LastReportedFingerprint,
		LastDeployRunID:         conv.LastDeployRunID,
		PendingConvergence:      conv.DesiredFingerprint != conv.AppliedFingerprint,
	}
	_ = appendRunRecord(r.StateDir, runID, RunRecord{RunID: runID, StartedAt: time.Now().UTC(), Phase: PhaseReport, Message: "post convergence"})
	if err := cl.PostConvergence(ctx, report); err != nil {
		return conv, fmt.Errorf("post convergence: %w", err)
	}
	conv.LastReportedFingerprint = conv.AppliedFingerprint
	conv.UpdatedAt = time.Now().UTC()
	conv.SchemaVersion = api.SchemaV1

	return conv, state.WriteJSONAtomic(paths.ConvergenceState(r.StateDir), conv, 0o644)
}

func appendRunRecord(stateDir, runID string, rec RunRecord) error {
	dir := paths.DeployRuns(stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, runID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(rec)
}
