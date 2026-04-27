// Package framework is the harness that drives protocol conformance
// ("golden") tests across every provider. Each provider contributes a
// directory of JSON fixtures; the harness spins up an httptest server
// that replays the fixture, points the adapter at it, and diffs the
// adapter's output against the expected platform-shaped response.
//
// Conceptually a single case is:
//
//	input.request.json        → the platform ChatRequest the user sends
//	upstream.response.json    → what the vendor returns (status + body)
//	expected.response.json    → what the gateway should produce
//
// Keeping one file per concern makes failures obvious in diff output:
// a schema drift in the adapter shows up as a mismatch between
// upstream.response.json and expected.response.json only.
package framework

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
)

// ChatCase is one golden case for the chat capability.
type ChatCase struct {
	Name    string
	Input   domain.ChatRequest
	Upstream UpstreamFixture
	Expect  ExpectedOutcome
}

// UpstreamFixture is the mock vendor response.
type UpstreamFixture struct {
	Status  int             `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage `json:"body"`
}

// ExpectedOutcome is what the platform-visible result should be.
// Either Response (success) or Error (non-2xx) must be set.
type ExpectedOutcome struct {
	Response *domain.ChatResponse `json:"response,omitempty"`
	Error    *ExpectedError       `json:"error,omitempty"`
}

// ExpectedError is the minimum envelope the harness checks on failures.
type ExpectedError struct {
	Kind string `json:"kind"`
	Code string `json:"code,omitempty"`
}

// ChatAdapterFactory returns a brand-new adapter that targets baseURL.
// Tests provide this so the harness can inject a local httptest URL
// without pulling in each provider package's internals.
type ChatAdapterFactory func(baseURL string) provider.ChatProvider

// RunChat walks every ChatCase under dir and exercises it against the
// factory-built adapter.
func RunChat(t *testing.T, dir string, factory ChatAdapterFactory) {
	t.Helper()
	cases, err := loadChatCases(dir)
	if err != nil {
		t.Fatalf("load cases from %s: %v", dir, err)
	}
	if len(cases) == 0 {
		t.Fatalf("no golden cases under %s", dir)
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range c.Upstream.Headers {
					w.Header().Set(k, v)
				}
				status := c.Upstream.Status
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = w.Write(c.Upstream.Body)
			}))
			defer ts.Close()

			adapter := factory(ts.URL)
			if c.Expect.Error != nil {
				assertChatError(t, adapter, c)
				return
			}
			assertChatResponse(t, adapter, c)
		})
	}
}

func assertChatResponse(t *testing.T, adapter provider.ChatProvider, c ChatCase) {
	t.Helper()
	ctx := context.Background()
	req, err := adapter.TranslateRequest(ctx, &c.Input, domain.Credential{APIKey: "test"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upstream call: %v", err)
	}
	got, _, err := adapter.ParseResponse(ctx, resp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Object != c.Expect.Response.Object {
		t.Errorf("object: got %q want %q", got.Object, c.Expect.Response.Object)
	}
	if got.Model != c.Expect.Response.Model {
		t.Errorf("model: got %q want %q", got.Model, c.Expect.Response.Model)
	}
	if got.Usage.TotalTokens != c.Expect.Response.Usage.TotalTokens {
		t.Errorf("usage.total_tokens: got %d want %d", got.Usage.TotalTokens, c.Expect.Response.Usage.TotalTokens)
	}
	if len(got.Choices) != len(c.Expect.Response.Choices) {
		t.Fatalf("choices count: got %d want %d", len(got.Choices), len(c.Expect.Response.Choices))
	}
	for i, ch := range got.Choices {
		want := c.Expect.Response.Choices[i]
		if ch.FinishReason != want.FinishReason {
			t.Errorf("choice[%d].finish_reason: got %q want %q", i, ch.FinishReason, want.FinishReason)
		}
		gotContent, _ := ch.Message.Content.(string)
		wantContent, _ := want.Message.Content.(string)
		if gotContent != wantContent {
			t.Errorf("choice[%d].content: got %q want %q", i, gotContent, wantContent)
		}
	}
}

func assertChatError(t *testing.T, adapter provider.ChatProvider, c ChatCase) {
	t.Helper()
	ctx := context.Background()
	req, err := adapter.TranslateRequest(ctx, &c.Input, domain.Credential{APIKey: "test"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upstream call: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ue := adapter.MapError(resp.StatusCode, body)
	if ue == nil {
		t.Fatalf("expected UnifiedError, got nil")
	}
	if string(ue.Kind) != c.Expect.Error.Kind {
		t.Errorf("error.kind: got %q want %q", ue.Kind, c.Expect.Error.Kind)
	}
	if c.Expect.Error.Code != "" && !strings.Contains(ue.Code, c.Expect.Error.Code) {
		t.Errorf("error.code: got %q want substring %q", ue.Code, c.Expect.Error.Code)
	}
}

func loadChatCases(dir string) ([]ChatCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []ChatCase
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".case.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var c ChatCase
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		if c.Name == "" {
			c.Name = strings.TrimSuffix(name, ".case.json")
		}
		out = append(out, c)
	}
	return out, nil
}
