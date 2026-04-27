package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/llmhub/llmhub/internal/billing/client"
	"github.com/llmhub/llmhub/internal/billing/rpc"
)

func TestFreezeAccepted(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "s3cret" {
			t.Errorf("missing token header")
		}
		_ = json.NewEncoder(w).Encode(rpc.FreezeResponse{Accepted: true})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "s3cret")
	if err := c.Freeze(context.Background(), "req-1", 42, 100); err != nil {
		t.Fatalf("freeze: %v", err)
	}
}

func TestFreezeInsufficient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(rpc.FreezeResponse{Accepted: false, Reason: "insufficient_balance"})
	}))
	defer srv.Close()
	c := client.New(srv.URL, "tok")
	err := c.Freeze(context.Background(), "req-2", 42, 100)
	if !errors.Is(err, client.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestSettleRoundTrip(t *testing.T) {
	t.Parallel()
	got := make(chan rpc.SettleRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpc.SettleRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		got <- req
		_ = json.NewEncoder(w).Encode(rpc.Empty{})
	}))
	defer srv.Close()
	c := client.New(srv.URL, "tok")
	if err := c.Settle(context.Background(), "req-3", 12); err != nil {
		t.Fatalf("settle: %v", err)
	}
	r := <-got
	if r.RequestID != "req-3" || r.ActualCents != 12 {
		t.Fatalf("payload mismatch: %+v", r)
	}
}
