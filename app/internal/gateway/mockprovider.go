package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/llmhub/llmhub/internal/capability/chat"
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/platform/id"
)

// MockProvider is a ProviderInvoker used in M3/early-M4 dev. It echoes
// the last user message with a canned prefix. The real upstream flow
// goes through ProviderDispatcher.
type MockProvider struct{}

// Invoke satisfies chat.ProviderInvoker for non-streaming requests.
func (MockProvider) Invoke(_ context.Context, providerID string, _ int64, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	reply := mockReply(providerID, req)
	in := len(lastContent(req)) / 3
	out := len(reply) / 3
	return &domain.ChatResponse{
		ID:      id.Prefixed("chatcmpl"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []domain.ChatChoice{
			{
				Index:   0,
				Message: domain.ChatMessage{Role: "assistant", Content: reply},
				FinishReason: "stop",
			},
		},
		Usage: domain.Usage{InputTokens: in, OutputTokens: out, TotalTokens: in + out},
	}, nil
}

// InvokeStream emits the reply as a sequence of small SSE chunks so we
// can exercise the streaming path without a live upstream.
func (MockProvider) InvokeStream(ctx context.Context, providerID string, _ int64, req *domain.ChatRequest, w chat.StreamWriter) (*domain.Usage, error) {
	reply := mockReply(providerID, req)
	words := strings.Split(reply, " ")

	for i, token := range words {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		chunk := map[string]any{
			"id":      "chatcmpl-mock",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{"content": token + " "},
				},
			},
		}
		if i == len(words)-1 {
			chunk["choices"].([]map[string]any)[0]["finish_reason"] = "stop"
		}
		b, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		w.Flush()
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	w.Flush()

	in := len(lastContent(req)) / 3
	out := len(reply) / 3
	return &domain.Usage{InputTokens: in, OutputTokens: out, TotalTokens: in + out}, nil
}

func mockReply(providerID string, req *domain.ChatRequest) string {
	return "[mock:" + providerID + "] echo: " + strings.TrimSpace(lastContent(req))
}

func lastContent(req *domain.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if s, ok := req.Messages[i].Content.(string); ok {
			return s
		}
	}
	return ""
}
