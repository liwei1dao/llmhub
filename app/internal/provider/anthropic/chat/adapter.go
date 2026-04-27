// Package chat is the Anthropic Messages API adapter.
//
// Anthropic does NOT speak OpenAI: the request shape uses a separate
// `system` string + `messages` (no system role), and the response
// returns content as a list of content-blocks. The adapter therefore
// translates both directions instead of passing through.
//
// References:
//   - https://docs.anthropic.com/en/api/messages
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
)

// Adapter implements provider.ChatProvider for Anthropic.
type Adapter struct {
	cfg provider.Config
}

// New constructs an Adapter from provider config.
func New(cfg provider.Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	return &Adapter{cfg: cfg}
}

// AnthropicVersion is the API version header value sent on every call.
// Pinning here makes upstream-side changes a deliberate update.
const AnthropicVersion = "2023-06-01"

// upstreamReq is the platform → Anthropic body shape.
type upstreamReq struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
	Temperature *float32         `json:"temperature,omitempty"`
	TopP        *float32         `json:"top_p,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`    // user / assistant
	Content string `json:"content"` // simplified — text only for M10
}

// upstreamResp is the Anthropic Messages response shape.
type upstreamResp struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Model      string             `json:"model"`
	Content    []responseContent  `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

type responseContent struct {
	Type string `json:"type"` // text / tool_use
	Text string `json:"text,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// TranslateRequest converts a platform ChatRequest into Anthropic's
// /v1/messages body. The system prompt — if any — is lifted out of
// messages into the top-level field; tool calls are deferred to V2.
func (a *Adapter) TranslateRequest(ctx context.Context, req *domain.ChatRequest, cred domain.Credential) (*http.Request, error) {
	if req == nil {
		return nil, fmt.Errorf("anthropic/chat: nil request")
	}
	body := upstreamReq{
		Model:       req.Model,
		MaxTokens:   defaultIfZero(intDeref(req.MaxTokens), 1024),
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	for _, m := range req.Messages {
		text, _ := m.Content.(string)
		switch m.Role {
		case "system":
			if body.System != "" {
				body.System += "\n\n"
			}
			body.System += text
		case "user", "assistant":
			body.Messages = append(body.Messages, anthropicMessage{Role: m.Role, Content: text})
		}
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := a.cfg.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", AnthropicVersion)
	if cred.APIKey != "" {
		httpReq.Header.Set("x-api-key", cred.APIKey)
	}
	return httpReq, nil
}

// ParseResponse converts an Anthropic non-streaming response back to
// the platform's OpenAI-shaped ChatResponse.
func (a *Adapter) ParseResponse(_ context.Context, resp *http.Response) (*domain.ChatResponse, *domain.Usage, error) {
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	var up upstreamResp
	if err := json.Unmarshal(raw, &up); err != nil {
		return nil, nil, fmt.Errorf("anthropic parse: %w", err)
	}
	var text string
	for _, c := range up.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	usage := domain.Usage{
		InputTokens:  up.Usage.InputTokens,
		OutputTokens: up.Usage.OutputTokens,
		TotalTokens:  up.Usage.InputTokens + up.Usage.OutputTokens,
	}
	out := &domain.ChatResponse{
		ID:     up.ID,
		Object: "chat.completion",
		Model:  up.Model,
		Choices: []domain.ChatChoice{
			{
				Index:        0,
				Message:      domain.ChatMessage{Role: "assistant", Content: text},
				FinishReason: mapStopReason(up.StopReason),
			},
		},
		Usage: usage,
	}
	return out, &usage, nil
}

// TranslateStreamChunk: Anthropic SSE uses a different event-frame
// convention (event: ... / data: ...) than OpenAI. M11 will normalize
// the chunks; for M10 we forward them as-is so callers wanting the
// raw stream can opt in via a future header flag.
func (a *Adapter) TranslateStreamChunk(raw []byte) ([]byte, *domain.Usage, error) {
	return raw, nil, nil
}

// MapError converts an Anthropic error body into a UnifiedError.
func (a *Adapter) MapError(statusCode int, body []byte) *domain.UnifiedError {
	var up struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &up)
	switch up.Error.Type {
	case "rate_limit_error":
		return domain.NewError(domain.ErrRateLimited, "LLMH_429_ANTH_rate_limit_error", up.Error.Message)
	case "authentication_error", "permission_error":
		return domain.NewError(domain.ErrUnauthorized, "LLMH_401_ANTH_"+up.Error.Type, up.Error.Message)
	case "invalid_request_error":
		return domain.NewError(domain.ErrInvalidRequest, "LLMH_400_ANTH_invalid_request_error", up.Error.Message)
	case "overloaded_error":
		return domain.NewError(domain.ErrUpstreamError, "LLMH_503_ANTH_overloaded_error", up.Error.Message)
	}
	switch {
	case statusCode == http.StatusTooManyRequests:
		return domain.NewError(domain.ErrRateLimited, "LLMH_429_ANTH", "upstream rate limited")
	case statusCode == http.StatusUnauthorized:
		return domain.NewError(domain.ErrUnauthorized, "LLMH_401_ANTH", "upstream unauthorized")
	case statusCode >= 500:
		return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_ANTH", statusCode), string(body))
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_ANTH", statusCode), string(body))
}

// ----- helpers -----

func mapStopReason(r string) string {
	switch r {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	}
	return r
}

func intDeref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func defaultIfZero(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
