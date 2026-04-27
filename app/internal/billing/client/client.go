// Package client is the gateway-side stub for the remote billing
// service. It implements the chat.BillingClient interface over HTTP
// so callers can swap between the in-process wallet and the remote
// service without touching business logic.
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

	"github.com/llmhub/llmhub/internal/billing/rpc"
)

// Client talks to a remote billing server over HTTP/JSON.
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
		HTTP: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ErrInsufficientBalance mirrors wallet.ErrInsufficientFunds so callers
// can handle the "not enough money" outcome with a stable error value.
var ErrInsufficientBalance = errors.New("billing: insufficient balance")

// Freeze pre-authorizes cents.
func (c *Client) Freeze(ctx context.Context, requestID string, userID, cents int64) error {
	var out rpc.FreezeResponse
	if err := c.do(ctx, "Freeze", rpc.FreezeRequest{
		RequestID: requestID, UserID: userID, Cents: cents,
	}, &out); err != nil {
		return err
	}
	if !out.Accepted {
		if out.Reason == "insufficient_balance" {
			return ErrInsufficientBalance
		}
		return fmt.Errorf("billing rejected: %s", out.Reason)
	}
	return nil
}

// Settle finalizes the hold.
func (c *Client) Settle(ctx context.Context, requestID string, cents int64) error {
	return c.do(ctx, "Settle", rpc.SettleRequest{
		RequestID: requestID, ActualCents: cents,
	}, &rpc.Empty{})
}

// Release frees a hold with no charge.
func (c *Client) Release(ctx context.Context, requestID string) error {
	return c.do(ctx, "Release", rpc.ReleaseRequest{RequestID: requestID}, &rpc.Empty{})
}

func (c *Client) do(ctx context.Context, method string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/rpc/billing/%s", c.BaseURL, method)
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
		return fmt.Errorf("billing call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("billing %s: %d %s", method, resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
