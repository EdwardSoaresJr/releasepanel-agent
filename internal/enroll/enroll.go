package enroll

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"releasepanel/agent/internal/central"
	"releasepanel/agent/internal/config"
	"releasepanel/agent/internal/paths"
	"releasepanel/agent/internal/state"
	"releasepanel/agent/pkg/api"
)

func Run(ctx context.Context, cfg *config.Config, centralBaseURL, token string, facts api.NodeFacts) error {
	cl, err := central.New(centralBaseURL, cfg.SkipTLSVerify)
	if err != nil {
		return err
	}

	req := api.EnrollRequest{
		SchemaVersion: api.SchemaV1,
		Token:         token,
		NodeFacts:     facts,
	}

	resp, err := cl.Enroll(ctx, req)
	if err != nil {
		return fmt.Errorf("enroll request: %w", err)
	}
	if strings.TrimSpace(resp.NodeID) == "" || strings.TrimSpace(resp.APIKey) == "" {
		return fmt.Errorf("enroll response missing node_id or api_key")
	}

	rec := state.EnrollmentRecord{
		SchemaVersion:  api.SchemaV1,
		NodeID:         resp.NodeID,
		APIKey:         resp.APIKey,
		CentralBaseURL: strings.TrimRight(centralBaseURL, "/"),
		EnrolledAt:     time.Now().UTC(),
	}

	path := paths.Enrollment(cfg.StateDir)
	if err := state.EnsureTree(0o755, cfg.StateDir); err != nil {
		return err
	}
	if err := state.WriteJSONAtomic(path, rec, 0o600); err != nil {
		return fmt.Errorf("persist enrollment: %w", err)
	}
	return nil
}

func ReadRecord(stateDir string) (*state.EnrollmentRecord, error) {
	path := paths.Enrollment(stateDir)
	var rec state.EnrollmentRecord
	if err := state.ReadJSON(path, &rec); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", state.ErrNotExist, path)
		}
		return nil, err
	}
	return &rec, nil
}
