package gateway

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability/chat"
	"github.com/llmhub/llmhub/internal/domain"
)

// RealProvider is a chat.ProviderInvoker backed by ProviderDispatcher.
// Use this in production; MockProvider is for dev smoke tests only.
type RealProvider struct {
	D *ProviderDispatcher
}

// Invoke runs a non-streaming chat call through the dispatcher.
func (r RealProvider) Invoke(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	return r.D.InvokeChat(ctx, providerID, accountID, req)
}

// InvokeStream runs a streaming chat call through the dispatcher.
func (r RealProvider) InvokeStream(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest, w chat.StreamWriter) (*domain.Usage, error) {
	return r.D.InvokeChatStream(ctx, providerID, accountID, req, w)
}
