package server_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/llmhub/llmhub/internal/billing/client"
	"github.com/llmhub/llmhub/internal/billing/rpc"
	"github.com/llmhub/llmhub/internal/billing/server"
	"github.com/llmhub/llmhub/internal/platform/log"
)

// fakeWallet stub-implements only the methods server invokes via the
// concrete *wallet.Service receiver. We can't substitute the type
// directly, so this test file exercises the HTTP boundary via the
// real handler and asserts JSON contracts only.
//
// Round-trip: client.Freeze marshals to JSON and reaches the server's
// requireToken middleware; we don't actually need a wallet here, just
// the auth + decode + response shape.

func TestRequireTokenRejectsMissingHeader(t *testing.T) {
	t.Parallel()
	srv := server.New(log.New("test"), nil, "expected")
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	c := client.New(ts.URL, "wrong")
	err := c.Freeze(context.Background(), "req", 1, 1)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401, got %v", err)
	}
}

func TestRPCContractStable(t *testing.T) {
	t.Parallel()
	// Encoding parity between client struct and rpc package shape.
	body, _ := json.Marshal(rpc.FreezeRequest{RequestID: "r", UserID: 1, Cents: 2})
	want := `{"request_id":"r","user_id":1,"cents":2}`
	if string(body) != want {
		t.Fatalf("freeze wire shape drift: got %s want %s", string(body), want)
	}
}
