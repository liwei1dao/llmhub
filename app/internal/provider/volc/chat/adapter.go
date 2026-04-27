// Package chat is the Volcengine Ark chat adapter.
//
// Ark speaks an OpenAI-compatible dialect: same JSON shape for
// /chat/completions, same SSE frames. The adapter therefore forwards
// the platform-shaped request nearly verbatim, and maps error bodies
// via the operator-maintained error_mapping in configs/providers/volc.yaml.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
)

// Adapter implements provider.ChatProvider for Volcengine Ark.
type Adapter struct {
	cfg provider.Config
}

// New constructs an Adapter from provider config.
func New(cfg provider.Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	if cfg.Auth.Header == "" {
		cfg.Auth.Header = "Authorization"
	}
	return &Adapter{cfg: cfg}
}

// TranslateRequest builds the upstream HTTP request. Ark's schema is a
// superset of OpenAI's, so the body is passed through after the caller
// has already swapped in the upstream model id.
func (a *Adapter) TranslateRequest(ctx context.Context, req *domain.ChatRequest, cred domain.Credential) (*http.Request, error) {
	if req == nil {
		return nil, fmt.Errorf("volc/chat: nil request")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	url := a.cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cred.APIKey != "" {
		httpReq.Header.Set(a.cfg.Auth.Header, "Bearer "+cred.APIKey)
	}
	return httpReq, nil
}

// ParseResponse decodes a non-streaming Ark response into the platform shape.
func (a *Adapter) ParseResponse(_ context.Context, resp *http.Response) (*domain.ChatResponse, *domain.Usage, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	var out domain.ChatResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}
	return &out, &out.Usage, nil
}

// streamChunk is the shape of one SSE data frame. We only care about
// fields that affect platform accounting (usage) — the rest is passed
// through opaquely to the client.
type streamChunk struct {
	Usage *domain.Usage `json:"usage,omitempty"`
}

// TranslateStreamChunk passes Ark's SSE chunks through unchanged and
// extracts usage from the final chunk when the server sends it
// (Ark emits the same `usage` field as OpenAI on the terminal chunk).
func (a *Adapter) TranslateStreamChunk(raw []byte) ([]byte, *domain.Usage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw, nil, nil
	}
	var probe streamChunk
	if err := json.Unmarshal(trimmed, &probe); err == nil && probe.Usage != nil {
		return raw, probe.Usage, nil
	}
	return raw, nil, nil
}

// upstreamError is the shape Ark uses for non-2xx bodies.
type upstreamError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// MapError converts an upstream error body into a platform UnifiedError.
// The mapping table lives in configs/providers/volc.yaml so operators
// can refine it without recompiling.
func (a *Adapter) MapError(statusCode int, body []byte) *domain.UnifiedError {
	var up upstreamError
	if err := json.Unmarshal(body, &up); err == nil && up.Error.Code != "" {
		if kind, ok := a.cfg.ErrorMapping[up.Error.Code]; ok {
			return buildError(kind, up.Error.Code, up.Error.Message, statusCode)
		}
	}
	if len(body) > 0 {
		lower := strings.ToLower(string(body))
		for needle, kind := range a.cfg.ErrorMapping {
			if strings.Contains(lower, strings.ToLower(needle)) {
				return buildError(kind, needle, string(body), statusCode)
			}
		}
	}
	return statusDefault(statusCode, body)
}

func buildError(kind, code, message string, statusCode int) *domain.UnifiedError {
	switch kind {
	case "rate_limited":
		return domain.NewError(domain.ErrRateLimited, "LLMH_429_VOLC_"+code, message)
	case "depleted":
		return domain.NewError(domain.ErrInsufficientBalance, "LLMH_402_VOLC_"+code, message)
	case "banned":
		return domain.NewError(domain.ErrUnauthorized, "LLMH_401_VOLC_"+code, message)
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_VOLC_%s", statusCode, code), message)
}

func statusDefault(statusCode int, body []byte) *domain.UnifiedError {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return domain.NewError(domain.ErrRateLimited, "LLMH_429_VOLC", "upstream rate limited")
	case statusCode == http.StatusUnauthorized:
		return domain.NewError(domain.ErrUnauthorized, "LLMH_401_VOLC", "upstream unauthorized")
	case statusCode == http.StatusPaymentRequired:
		return domain.NewError(domain.ErrInsufficientBalance, "LLMH_402_VOLC", "upstream balance exhausted")
	case statusCode >= 500:
		return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_VOLC", statusCode), string(body))
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_VOLC", statusCode), string(body))
}
