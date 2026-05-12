package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func EnsureTree(mode fs.FileMode, dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, mode); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func WriteJSONAtomic(path string, v any, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".rp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
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
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func ReadJSON(path string, dest any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

// EnrollmentRecord is persisted credentials after successful enrollment.
type EnrollmentRecord struct {
	SchemaVersion   int       `json:"schema_version"`
	NodeID          string    `json:"node_id"`
	APIKey          string    `json:"api_key"`
	CentralBaseURL  string    `json:"central_base_url"`
	EnrolledAt      time.Time `json:"enrolled_at"`
}

// RuntimeCounters holds coarse agent loop metadata (inspectable, not authoritative for central).
type RuntimeCounters struct {
	SchemaVersion       int       `json:"schema_version"`
	StartedAt           time.Time `json:"started_at"`
	LastLoopAt          time.Time `json:"last_loop_at"`
	LoopCount           int64     `json:"loop_count"`
	LastInventoryPostAt time.Time `json:"last_inventory_post_at,omitempty"`
	LastHealthPostAt    time.Time `json:"last_health_post_at,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
}

// ConvergenceRecord tracks fingerprint progression on the node.
type ConvergenceRecord struct {
	SchemaVersion           int       `json:"schema_version"`
	DesiredFingerprint      string    `json:"desired_fingerprint,omitempty"`
	AppliedFingerprint      string    `json:"applied_fingerprint,omitempty"`
	LastReportedFingerprint string    `json:"last_reported_fingerprint,omitempty"`
	LastDeployRunID         string    `json:"last_deploy_run_id,omitempty"`
	UpdatedAt               time.Time `json:"updated_at"`
}

var ErrNotExist = errors.New("state file does not exist")
