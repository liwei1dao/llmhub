// Package client is the gateway-side stub for the remote scheduler
// service. It implements the same Pick/Report surface as the in-process
// scheduler.Service so the chat handler doesn't care which one is
// wired in.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/internal/scheduler/rpc"
)

// Client talks to a remote scheduler over HTTP/JSON.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New returns a Client with sensible timeouts.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
}

// ErrNoAccountAvailable is the typed equivalent of HTTP 503 with kind=no_account_available.
var ErrNoAccountAvailable = errors.New("scheduler: no account available")

// Pick mirrors scheduler.Service.Pick.
func (c *Client) Pick(ctx context.Context, req scheduler.PickRequest) (*scheduler.PickResult, error) {
	body := rpc.PickRequest{
		RequestID:         req.RequestID,
		UserID:            req.UserID,
		CapabilityID:      req.CapabilityID,
		ProviderID:        req.ProviderID,
		ModelID:           req.ModelID,
		EstimatedUnits:    req.EstimatedUnits,
		RiskLevel:         int(req.RiskLevel),
		SessionKey:        req.SessionKey,
		ExcludeAccountIDs: req.ExcludeAccountIDs,
	}
	var out rpc.PickResponse
	if err := c.do(ctx, "Pick", body, &out); err != nil {
		return nil, err
	}
	return &scheduler.PickResult{
		AccountID:  out.AccountID,
		ProviderID: out.ProviderID,
		Tier:       domain.Tier(out.Tier),
		PickToken:  out.PickToken,
	}, nil
}

// Report mirrors scheduler.Service.Report.
func (c *Client) Report(ctx context.Context, accountID int64, r scheduler.ReportResult) error {
	return c.do(ctx, "Report", rpc.ReportRequest{
		AccountID: accountID,
		Result:    string(r),
	}, &rpc.Empty{})
}

func (c *Client) do(ctx context.Context, method string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/rpc/scheduler/%s", c.BaseURL, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("X-Internal-Token", c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("scheduler call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return ErrNoAccountAvailable
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scheduler %s: %d %s", method, resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
