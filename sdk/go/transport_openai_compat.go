package llmhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openaiCompat handles vendors whose chat-completions endpoint
// matches OpenAI's contract (DeepSeek, Volc 方舟, Aliyun DashScope's
// compat mode, Moonshot, ...). The lease's Endpoint is the base URL;
// we POST /chat/completions on it with `Authorization: Bearer <key>`.
//
// Auth resolution rules (so a single transport handles a few different
// catalog entries):
//   - auth_payload["api_key"]    — DeepSeek / DashScope / Moonshot
//   - auth_payload["app_token"]  — Volc Ark (also accepts api_key)
//
// Whichever is present wins; api_key is the canonical key in the SDK's
// internal contract because the platform normalises Volc auth into it
// where possible.
type openaiCompat struct{}

func init() {
	registerTransport("openai_compat", openaiCompat{})
	registerTransport("openai_native", openaiCompat{})
}

func (openaiCompat) invokeChat(
	ctx context.Context, c *Client, lease *Lease, req *ChatRequest,
) (*ChatResponse, time.Duration, error) {
	httpReq, err := buildOpenAIChatReq(ctx, c, lease, req)
	if err != nil {
		return nil, 0, err
	}
	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, 0, &APIError{Status: 0, Code: "network_error", Message: err.Error(), Source: "upstream"}
	}
	ttfb := time.Since(start)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ttfb, makeUpstreamError(resp.StatusCode, body)
	}
	var out ChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, ttfb, &APIError{
			Status: resp.StatusCode, Code: "parse_error",
			Message: fmt.Sprintf("decode upstream response: %v", err),
			Source:  "upstream",
		}
	}
	return &out, ttfb, nil
}

func (openaiCompat) invokeChatStream(
	ctx context.Context, c *Client, lease *Lease, req *ChatRequest, out *Stream,
) (*ChatUsage, time.Duration, error) {
	httpReq, err := buildOpenAIChatReq(ctx, c, lease, req)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, 0, &APIError{Status: 0, Code: "network_error", Message: err.Error(), Source: "upstream"}
	}
	ttfb := time.Since(start)
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, ttfb, makeUpstreamError(resp.StatusCode, body)
	}

	var lastUsage *ChatUsage
	err = readSSE(resp.Body, func(payload []byte) error {
		var frame struct {
			Choices []struct {
				Delta        ChatMessage `json:"delta"`
				FinishReason string      `json:"finish_reason"`
			} `json:"choices"`
			Usage *ChatUsage `json:"usage"`
		}
		_ = json.Unmarshal(payload, &frame)
		chunk := StreamChunk{Raw: append([]byte(nil), payload...)}
		if len(frame.Choices) > 0 {
			d := frame.Choices[0].Delta
			chunk.Delta = &d
		}
		if frame.Usage != nil {
			chunk.Usage = frame.Usage
			lastUsage = frame.Usage
		}
		select {
		case out.chunks <- chunk:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		return lastUsage, ttfb, &APIError{
			Status: resp.StatusCode, Code: "stream_error",
			Message: err.Error(), Source: "upstream",
		}
	}
	// Emit a final Done frame so receivers can distinguish a clean
	// close from a network drop without inspecting Stream.Err.
	select {
	case out.chunks <- StreamChunk{Done: true, Usage: lastUsage}:
	case <-ctx.Done():
	}
	return lastUsage, ttfb, nil
}

func buildOpenAIChatReq(ctx context.Context, c *Client, lease *Lease, req *ChatRequest) (*http.Request, error) {
	model := lease.UpstreamModel
	if model == "" {
		// Volc Ark allows specifying the endpoint id (ep-...) via the
		// auth_payload's app_id when no SKU upstream model is set.
		model = lease.AuthPayload["app_id"]
	}
	if model == "" {
		return nil, fmt.Errorf("llmhub: lease %s has no upstream_model and no app_id fallback", lease.LeaseID)
	}
	body, err := mergeBody(req, model)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(lease.Endpoint, "/") + "/chat/completions"
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", c.cfg.UserAgent)
	if token := pickToken(lease.AuthPayload); token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	} else {
		return nil, fmt.Errorf("llmhub: lease %s has no usable bearer token in auth_payload", lease.LeaseID)
	}
	return r, nil
}

// pickToken extracts the bearer the upstream wants. Order matters
// because Volc Ark accepts both `app_token` (canonical) and `api_key`
// (some operators alias it).
func pickToken(payload map[string]string) string {
	for _, key := range []string{"app_token", "api_key"} {
		if v := strings.TrimSpace(payload[key]); v != "" {
			return v
		}
	}
	return ""
}
