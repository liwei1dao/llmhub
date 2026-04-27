// Package openai_compat provides reusable building blocks for any
// upstream vendor whose chat API mirrors OpenAI's /v1/chat/completions
// contract. Concrete provider adapters (DeepSeek, Ark, Moonshot, ...)
// embed ChatAdapter and override the bits that differ (auth,
// base_url, error-code dictionary).
package openai_compat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/llmhub/llmhub/internal/domain"
)

// ChatAdapter implements provider.ChatProvider against any upstream
// whose chat endpoint is OpenAI-compatible.
type ChatAdapter struct {
	ProviderTag  string             // short tag embedded in error codes, e.g. "DEEPSEEK"
	BaseURL      string             // without trailing slash
	AuthHeader   string             // typically "Authorization"
	ErrorMap     map[string]string  // upstream error code → platform kind
}

// TranslateRequest serializes the platform request verbatim — the
// caller must have already swapped model ids.
func (a *ChatAdapter) TranslateRequest(ctx context.Context, req *domain.ChatRequest, cred domain.Credential) (*http.Request, error) {
	if req == nil {
		return nil, fmt.Errorf("openai_compat/chat: nil request")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	url := strings.TrimRight(a.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	h := a.AuthHeader
	if h == "" {
		h = "Authorization"
	}
	if cred.APIKey != "" {
		httpReq.Header.Set(h, "Bearer "+cred.APIKey)
	}
	return httpReq, nil
}

// ParseResponse decodes a non-streaming response.
func (a *ChatAdapter) ParseResponse(_ context.Context, resp *http.Response) (*domain.ChatResponse, *domain.Usage, error) {
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

type streamChunk struct {
	Usage *domain.Usage `json:"usage,omitempty"`
}

// TranslateStreamChunk forwards chunks untouched and lifts usage
// from the terminal frame when the upstream provides it.
func (a *ChatAdapter) TranslateStreamChunk(raw []byte) ([]byte, *domain.Usage, error) {
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

type upstreamError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// MapError converts an upstream error into a platform UnifiedError
// using the per-provider ErrorMap (structured code match → body
// substring match → HTTP-status fallback).
func (a *ChatAdapter) MapError(statusCode int, body []byte) *domain.UnifiedError {
	var up upstreamError
	if err := json.Unmarshal(body, &up); err == nil && up.Error.Code != "" {
		if kind, ok := a.ErrorMap[up.Error.Code]; ok {
			return a.buildError(kind, up.Error.Code, up.Error.Message, statusCode)
		}
	}
	if len(body) > 0 {
		lower := strings.ToLower(string(body))
		for needle, kind := range a.ErrorMap {
			if strings.Contains(lower, strings.ToLower(needle)) {
				return a.buildError(kind, needle, string(body), statusCode)
			}
		}
	}
	return a.statusDefault(statusCode, body)
}

func (a *ChatAdapter) buildError(kind, code, message string, statusCode int) *domain.UnifiedError {
	tag := a.ProviderTag
	if tag == "" {
		tag = "UPSTREAM"
	}
	switch kind {
	case "rate_limited":
		return domain.NewError(domain.ErrRateLimited, fmt.Sprintf("LLMH_429_%s_%s", tag, code), message)
	case "depleted":
		return domain.NewError(domain.ErrInsufficientBalance, fmt.Sprintf("LLMH_402_%s_%s", tag, code), message)
	case "banned", "auth_failed":
		return domain.NewError(domain.ErrUnauthorized, fmt.Sprintf("LLMH_401_%s_%s", tag, code), message)
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_%s_%s", statusCode, tag, code), message)
}

func (a *ChatAdapter) statusDefault(statusCode int, body []byte) *domain.UnifiedError {
	tag := a.ProviderTag
	if tag == "" {
		tag = "UPSTREAM"
	}
	switch {
	case statusCode == http.StatusTooManyRequests:
		return domain.NewError(domain.ErrRateLimited, fmt.Sprintf("LLMH_429_%s", tag), "upstream rate limited")
	case statusCode == http.StatusUnauthorized:
		return domain.NewError(domain.ErrUnauthorized, fmt.Sprintf("LLMH_401_%s", tag), "upstream unauthorized")
	case statusCode == http.StatusPaymentRequired:
		return domain.NewError(domain.ErrInsufficientBalance, fmt.Sprintf("LLMH_402_%s", tag), "upstream balance exhausted")
	case statusCode >= 500:
		return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_%s", statusCode, tag), string(body))
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_%s", statusCode, tag), string(body))
}
