package provider

// Minimal Enori public-API client (X-Api-Key auth). STATUS: first-draft, uncompiled — see main.go.
// Base host is api.enori.io (the public REST API), NOT app.enori.io (the dashboard). See DESIGN.md §3.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.enori.io"

// Client is a thin wrapper over the Enori REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

// Monitor mirrors the Enori MonitorDto / CreateMonitorRequest (src/UpNest.Api/DTOs/MonitorDto.cs).
// MVP subset — extend to the full field set during the Go-verified build (DESIGN.md §2/§3).
type Monitor struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	GroupName       string `json:"groupName,omitempty"`
	URL             string `json:"url"`
	Type            string `json:"type"` // "website" | "ping" | "port" | "dns" | "domain" | "job" | "browser" | "apiflow"
	IntervalSeconds int64  `json:"intervalSeconds"`
	TimeoutSeconds  int64  `json:"timeoutSeconds,omitempty"`
	Paused          bool   `json:"paused,omitempty"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("enori API %s %s: HTTP %d: %s", method, path, resp.StatusCode, string(raw))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) CreateMonitor(ctx context.Context, m Monitor) (*Monitor, error) {
	var created Monitor
	if err := c.do(ctx, http.MethodPost, "/api/monitors", m, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (c *Client) GetMonitor(ctx context.Context, id string) (*Monitor, error) {
	var m Monitor
	if err := c.do(ctx, http.MethodGet, "/api/monitors/"+id, nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *Client) UpdateMonitor(ctx context.Context, id string, m Monitor) (*Monitor, error) {
	var updated Monitor
	if err := c.do(ctx, http.MethodPut, "/api/monitors/"+id, m, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (c *Client) DeleteMonitor(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/monitors/"+id, nil, nil)
}
