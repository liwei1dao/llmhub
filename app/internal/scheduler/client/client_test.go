package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/internal/scheduler/client"
	"github.com/llmhub/llmhub/internal/scheduler/rpc"
)

func TestPickRoundTrip(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			t.Errorf("missing token header")
		}
		var req rpc.PickRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.CapabilityID != "chat" {
			t.Errorf("capability mismatch: %s", req.CapabilityID)
		}
		_ = json.NewEncoder(w).Encode(rpc.PickResponse{
			AccountID: 7, ProviderID: "volc", Tier: "T1", PickToken: req.RequestID,
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	res, err := c.Pick(context.Background(), scheduler.PickRequest{
		RequestID: "r-1", UserID: 99, CapabilityID: "chat", ModelID: "x",
		RiskLevel: domain.RiskLow,
	})
	if err != nil {
		t.Fatalf("pick: %v", err)
	}
	if res.AccountID != 7 || res.ProviderID != "volc" || res.Tier != domain.TierT1 {
		t.Fatalf("unexpected: %+v", res)
	}
}

func TestPickNoAccount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"type":"no_account_available"}}`))
	}))
	defer srv.Close()
	c := client.New(srv.URL, "tok")
	_, err := c.Pick(context.Background(), scheduler.PickRequest{RequestID: "r"})
	if !errors.Is(err, client.ErrNoAccountAvailable) {
		t.Fatalf("want ErrNoAccountAvailable, got %v", err)
	}
}
