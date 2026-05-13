package central

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"releasepanel/agent/pkg/api"
)

const pathAgentPing = "/api/v1/agent/ping"

// PingClient calls Central Sanctum agent routes (Bearer token only — no node headers).
type PingClient struct {
	httpCli *http.Client
	baseURL *url.URL
	token   string
}

// NewPingClient reuses the same TLS/http tuning as New().
func NewPingClient(baseURL string, skipTLSVerify bool, bearerToken string) (*PingClient, error) {
	cli, err := New(baseURL, skipTLSVerify)
	if err != nil {
		return nil, err
	}
	tok := strings.TrimSpace(bearerToken)
	if tok == "" {
		return nil, fmt.Errorf("ping client: empty bearer token")
	}
	return &PingClient{
		httpCli: cli.http,
		baseURL: cli.baseURL,
		token:   tok,
	}, nil
}

// GetPing observes deploy intents without submitting runtime reports.
func (p *PingClient) GetPing(ctx context.Context) (*api.AgentPingResponse, error) {
	return p.doPing(ctx, http.MethodGet, nil)
}

// PostPing submits receipts (e.g. site_runtime_reports) and returns refreshed intents.
func (p *PingClient) PostPing(ctx context.Context, body *api.AgentPingPostBody) (*api.AgentPingResponse, error) {
	return p.doPing(ctx, http.MethodPost, body)
}

func (p *PingClient) doPing(ctx context.Context, method string, body *api.AgentPingPostBody) (*api.AgentPingResponse, error) {
	u := p.baseURL.ResolveReference(&url.URL{Path: pathAgentPing})

	var reqBody io.Reader
	if body != nil && method == http.MethodPost {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("central %s %s: status=%d body=%s", method, u.Redacted(), resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out api.AgentPingResponse
	if len(raw) == 0 {
		return &out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode ping response: %w", err)
	}
	return &out, nil
}
