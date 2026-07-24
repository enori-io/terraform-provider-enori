package provider

// Minimal Enori public-API client (X-Api-Key auth). STATUS: pre-alpha, compiles clean — see main.go.
// Base host is api.enori.io (the public REST API), NOT app.enori.io (the dashboard). See DESIGN.md §3.
//
// Endpoints (verified against MonitorsController on 2026-07-24):
//   POST   /api/monitors        Authorize monitors:write  → MonitorDto
//   GET    /api/monitors/{id}   Authorize monitors:read   → MonitorDto
//   PUT    /api/monitors/{id}   Authorize monitors:write  → MonitorDto (partial: null field = no change)
//   DELETE /api/monitors/{id}   Authorize monitors:write
//
// Wire notes: the API serializes with camelCase property names + JsonStringEnumConverter, so `type`
// is the enum member name as a string ("Website"), read case-insensitively (lowercase "website" on
// send is accepted). Optional fields are POINTERS so a nil omits the field entirely (`omitempty`)
// while an explicit false/0 is still sent — avoids the classic Go zero-value-vs-omitted ambiguity.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.enori.io"

// errNotFound is returned by GetMonitor when the API responds 404, so Read can drop the resource
// from state (drift-recovery) rather than erroring.
var errNotFound = errors.New("monitor not found")

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

// Monitor mirrors the Enori CreateMonitorRequest / UpdateMonitorRequest / MonitorDto for the
// fields the provider manages (the common cross-type + HTTP + alerting core). Optional fields are
// pointers/slices so nil omits them on the wire. Type-specific advanced config (browser steps,
// ApiFlow, DNS routing, device emulation, encrypted variables) is intentionally out of scope for
// v0.1.0 — see DESIGN.md §2.
type Monitor struct {
	// Identity / always present.
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// Enum member name as a string ("website" accepted on send; API returns "Website"). Omitted on
	// update (UpdateMonitorRequest has no Type — type is immutable; the resource marks it RequiresReplace).
	Type string `json:"type,omitempty"`

	// Response-only (never set on send → omitted).
	Status   string `json:"status,omitempty"`
	IsActive *bool  `json:"isActive,omitempty"`

	// Optional / API-defaulted (pointer → nil omits; explicit false/0 is sent).
	GroupName            *string  `json:"groupName,omitempty"`
	IntervalSeconds      *int64   `json:"intervalSeconds,omitempty"`
	TimeoutSeconds       *int64   `json:"timeoutSeconds,omitempty"`
	HTTPMethod           *string  `json:"httpMethod,omitempty"`
	ExpectedStatusCode   *int64   `json:"expectedStatusCode,omitempty"`
	ExpectedKeyword      *string  `json:"expectedKeyword,omitempty"`
	RequestBody          *string  `json:"requestBody,omitempty"`
	CustomUserAgent      *string  `json:"customUserAgent,omitempty"`
	FollowRedirects      *bool    `json:"followRedirects,omitempty"`
	Port                 *int64   `json:"port,omitempty"`
	SslExpiryWarningDays *int64   `json:"sslExpiryWarningDays,omitempty"`
	FailureThreshold     *int64   `json:"failureThreshold,omitempty"`
	AlertOnDown          *bool    `json:"alertOnDown,omitempty"`
	AlertOnRecovered     *bool    `json:"alertOnRecovered,omitempty"`
	AlertChannelIds      []string `json:"alertChannelIds,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
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
	err := c.do(ctx, http.MethodDelete, "/api/monitors/"+id, nil, nil)
	// A monitor already gone is a successful delete from Terraform's perspective.
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}
