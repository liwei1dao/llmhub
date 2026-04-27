package gateway

import (
	"context"

	"github.com/llmhub/llmhub/internal/domain"
)

// EmbeddingInvoker bridges capability/embedding.Invoker onto the
// ProviderDispatcher's lookup → vault → provider path.
type EmbeddingInvoker struct {
	D *ProviderDispatcher
}

// Invoke runs the upstream embedding call.
func (e EmbeddingInvoker) Invoke(ctx context.Context, providerID string, accountID int64, req *domain.EmbeddingRequest) (*domain.EmbeddingResponse, error) {
	return e.D.InvokeEmbedding(ctx, providerID, accountID, req)
}
