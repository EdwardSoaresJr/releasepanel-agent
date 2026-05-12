package central

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"releasepanel/agent/internal/urlorigin"
	"releasepanel/agent/pkg/api"
)

// HTTP paths are rooted under /api/v1 on Central. central_base_url must be
// scheme://host[:port] only — see docs/CENTRAL_API.md.
const (
	pathEnrollFmt          = "/api/v1/nodes/enroll"
	pathHeartbeatFmt       = "/api/v1/nodes/%s/heartbeat"
	pathDesiredFmt         = "/api/v1/nodes/%s/desired"
	pathReportInventoryFmt = "/api/v1/nodes/%s/reports/inventory"
	pathReportHealthFmt    = "/api/v1/nodes/%s/reports/health"
	pathReportConvFmt      = "/api/v1/nodes/%s/reports/convergence"
)

type Client struct {
	baseURL *url.URL
	http    *http.Client
	nodeID  string
	apiKey  string
}

func New(base string, skipTLSVerify bool) (*Client, error) {
	base = strings.TrimSpace(base)
	if err := urlorigin.ValidateHTTPOrigin(base); err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		MaxIdleConns:        64,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: skipTLSVerify,
		},
	}
	return &Client{
		baseURL: u,
		http: &http.Client{
			Timeout:   60 * time.Second,
			Transport: tr,
		},
	}, nil
}

func (c *Client) WithAuth(nodeID, apiKey string) *Client {
	cp := *c
	cp.nodeID = nodeID
	cp.apiKey = apiKey
	return &cp
}

func (c *Client) Enroll(ctx context.Context, req api.EnrollRequest) (*api.EnrollResponse, error) {
	var out api.EnrollResponse
	if err := c.postJSON(ctx, pathEnrollFmt, req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) FetchDesired(ctx context.Context) ([]byte, string, error) {
	if c.nodeID == "" || c.apiKey == "" {
		return nil, "", fmt.Errorf("central client missing auth")
	}
	path := fmt.Sprintf(pathDesiredFmt, url.PathEscape(c.nodeID))
	raw, err := c.getRaw(ctx, path)
	if err != nil {
		return nil, "", err
	}
	// Minimal parse for fingerprint if present in JSON.
	fp := ""
	var stub struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(raw, &stub); err == nil {
		fp = stub.Fingerprint
	}
	return raw, fp, nil
}

func (c *Client) PostInventory(ctx context.Context, report api.InventoryReport) error {
	path := fmt.Sprintf(pathReportInventoryFmt, url.PathEscape(c.nodeID))
	return c.postJSON(ctx, path, report, nil, true)
}

func (c *Client) PostHealth(ctx context.Context, report api.HealthReport) error {
	path := fmt.Sprintf(pathReportHealthFmt, url.PathEscape(c.nodeID))
	return c.postJSON(ctx, path, report, nil, true)
}

func (c *Client) PostHeartbeat(ctx context.Context, report api.HeartbeatReport) error {
	path := fmt.Sprintf(pathHeartbeatFmt, url.PathEscape(c.nodeID))
	return c.postJSON(ctx, path, report, nil, true)
}

func (c *Client) PostConvergence(ctx context.Context, report api.ConvergenceReport) error {
	path := fmt.Sprintf(pathReportConvFmt, url.PathEscape(c.nodeID))
	return c.postJSON(ctx, path, report, nil, true)
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any, auth bool) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := c.baseURL.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		if c.apiKey == "" {
			return fmt.Errorf("missing api key")
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		if c.nodeID != "" {
			req.Header.Set("X-Releasepanel-Node-Id", c.nodeID)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("central %s %s: status=%d body=%s", req.Method, u.Redacted(), resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out == nil {
		return nil
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) getRaw(ctx context.Context, path string) ([]byte, error) {
	u := c.baseURL.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Releasepanel-Node-Id", c.nodeID)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("central GET %s: status=%d body=%s", u.Redacted(), resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
