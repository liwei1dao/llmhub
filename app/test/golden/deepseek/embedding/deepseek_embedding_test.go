// Package embedding runs golden conformance cases for the DeepSeek
// embedding adapter. The harness here is small enough to inline; if a
// second provider gains golden coverage we'll lift it into
// test/golden/framework alongside RunChat.
package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
	dsEmb "github.com/llmhub/llmhub/internal/provider/deepseek/embedding"
)

type embeddingCase struct {
	Name     string          `json:"name"`
	Inputs   []string        `json:"inputs"`
	Model    string          `json:"model"`
	Upstream upstreamFixture `json:"upstream"`
	Expect   expectation     `json:"expect"`
}

type upstreamFixture struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

type expectation struct {
	Vectors     int    `json:"vectors,omitempty"`
	FirstDim    int    `json:"first_dim,omitempty"`
	InputTokens int    `json:"input_tokens,omitempty"`
	ErrorKind   string `json:"error_kind,omitempty"`
}

func TestGoldenDeepSeekEmbedding(t *testing.T) {
	cases, err := loadCases(".")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("no embedding cases")
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.Upstream.Status)
				_, _ = w.Write(c.Upstream.Body)
			}))
			defer ts.Close()

			adapter := dsEmb.New(provider.Config{
				ID:      "deepseek",
				BaseURL: ts.URL,
				Auth:    provider.AuthConfig{Mode: "bearer", Header: "Authorization"},
			})
			vecs, usage, ue := adapter.Embed(context.Background(), c.Inputs, c.Model, domain.Credential{APIKey: "test"})
			if c.Expect.ErrorKind != "" {
				if ue == nil {
					t.Fatalf("expected error kind %s, got nil", c.Expect.ErrorKind)
				}
				if string(ue.Kind) != c.Expect.ErrorKind {
					t.Fatalf("error kind: got %s want %s", ue.Kind, c.Expect.ErrorKind)
				}
				return
			}
			if ue != nil {
				t.Fatalf("unexpected error: %v", ue)
			}
			if c.Expect.Vectors > 0 && len(vecs) != c.Expect.Vectors {
				t.Errorf("vectors=%d want %d", len(vecs), c.Expect.Vectors)
			}
			if c.Expect.FirstDim > 0 && (len(vecs) == 0 || len(vecs[0]) != c.Expect.FirstDim) {
				t.Errorf("first_dim=%v want %d", lenOrZero(vecs), c.Expect.FirstDim)
			}
			if c.Expect.InputTokens > 0 && (usage == nil || usage.InputTokens != c.Expect.InputTokens) {
				t.Errorf("input_tokens mismatch: got %v want %d", usage, c.Expect.InputTokens)
			}
		})
	}
}

func lenOrZero(vs [][]float32) int {
	if len(vs) == 0 {
		return 0
	}
	return len(vs[0])
}

func loadCases(dir string) ([]embeddingCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []embeddingCase
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".case.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var c embeddingCase
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		if c.Name == "" {
			c.Name = strings.TrimSuffix(e.Name(), ".case.json")
		}
		out = append(out, c)
	}
	return out, nil
}
