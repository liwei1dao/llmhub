package gateway

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/platform/vault"
	"github.com/llmhub/llmhub/internal/pool"
	"github.com/llmhub/llmhub/internal/provider"
)

// ProviderLookup is the minimal surface the dispatcher needs to find
// a concrete Provider instance at call time. In production this is
// *provider.Manager; tests pass stubs.
type ProviderLookup interface {
	Lookup(id string) (provider.Provider, bool)
}

// ProviderDispatcher runs real upstream calls on behalf of capability
// handlers. It is the seam between the platform's logical view of a
// call (user, logical model, risk level) and the vendor-specific wire
// protocol.
type ProviderDispatcher struct {
	Catalog   *catalog.Service
	Pool      *pool.Service
	Providers ProviderLookup
	Vault     vault.Resolver
	HTTP      *http.Client
}

// InvokeChat is the non-streaming path. It returns the unified
// ChatResponse with usage populated, or a *domain.UnifiedError.
func (d *ProviderDispatcher) InvokeChat(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	httpReq, cap, err := d.buildRequest(ctx, providerID, accountID, req)
	if err != nil {
		return nil, err
	}
	resp, doErr := d.HTTP.Do(httpReq)
	if doErr != nil {
		return nil, domain.NewError(domain.ErrUpstreamError, "LLMH_502_002", "upstream call failed").WithCause(doErr)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if ue := cap.MapError(resp.StatusCode, body); ue != nil {
			return nil, ue
		}
		return nil, domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_UP", resp.StatusCode), string(body))
	}
	out, _, perr := cap.ParseResponse(ctx, resp)
	if perr != nil {
		return nil, domain.NewError(domain.ErrUpstreamError, "LLMH_502_003", "parse upstream response").WithCause(perr)
	}
	return out, nil
}

// InvokeChatStream is the SSE path. Each upstream chunk is translated
// into an OpenAI-shaped chunk and written to w. The returned Usage is
// the best estimate seen in the terminal chunk (may be zero if the
// upstream did not include usage).
func (d *ProviderDispatcher) InvokeChatStream(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest, w StreamWriter) (*domain.Usage, error) {
	httpReq, cap, err := d.buildRequest(ctx, providerID, accountID, req)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, doErr := d.HTTP.Do(httpReq)
	if doErr != nil {
		return nil, domain.NewError(domain.ErrUpstreamError, "LLMH_502_002", "upstream call failed").WithCause(doErr)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		if ue := cap.MapError(resp.StatusCode, body); ue != nil {
			return nil, ue
		}
		return nil, domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_UP", resp.StatusCode), string(body))
	}

	var usage *domain.Usage
	scanner := bufio.NewScanner(resp.Body)
	// SSE messages can exceed the default 64KB (vision responses, long
	// tool-call args). Give the scanner generous headroom.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			// SSE event boundary — flush to client.
			w.Flush()
			continue
		}
		if !strings.HasPrefix(string(line), "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(string(line), "data:"))
		if payload == "[DONE]" {
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			w.Flush()
			break
		}
		translated, u, err := cap.TranslateStreamChunk([]byte(payload))
		if err != nil {
			return usage, domain.NewError(domain.ErrUpstreamError, "LLMH_502_004", "stream chunk parse").WithCause(err)
		}
		if u != nil {
			usage = u
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", translated); err != nil {
			return usage, err
		}
		w.Flush()
	}
	if err := scanner.Err(); err != nil {
		return usage, domain.NewError(domain.ErrUpstreamTimeout, "LLMH_504_001", "stream read").WithCause(err)
	}
	return usage, nil
}

// InvokeEmbedding routes an embedding request through the same
// catalog-resolve → vault → provider pipeline that chat uses, but
// without streaming.
func (d *ProviderDispatcher) InvokeEmbedding(ctx context.Context, providerID string, accountID int64, req *domain.EmbeddingRequest) (*domain.EmbeddingResponse, error) {
	prov, ok := d.Providers.Lookup(providerID)
	if !ok {
		return nil, domain.NewError(domain.ErrInternal, "LLMH_500_PROV", "provider not registered").WithMeta("provider", providerID)
	}
	cap := prov.EmbeddingCapability()
	if cap == nil {
		return nil, domain.NewError(domain.ErrInternal, "LLMH_500_NOCAP", "provider missing embedding capability").WithMeta("provider", providerID)
	}
	upstream, err := d.Catalog.ResolveUpstreamModel(ctx, req.Model, providerID)
	if err != nil {
		return nil, domain.NewError(domain.ErrModelNotFound, "LLMH_404_001", "model mapping missing").WithCause(err)
	}
	key, err := d.Pool.ActiveAPIKey(ctx, accountID, "embedding")
	if err != nil {
		// Embedding sometimes shares a chat-scoped key — try again.
		if key, err = d.Pool.ActiveAPIKey(ctx, accountID, "all"); err != nil {
			return nil, domain.NewError(domain.ErrInternal, "LLMH_500_NOKEY", "no active credential").WithCause(err)
		}
	}
	sec, err := d.Vault.Resolve(ctx, key.VaultRef)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternal, "LLMH_500_VAULT", "credential lookup failed").WithCause(err)
	}
	cred := domain.Credential{
		ProviderID: providerID,
		AccountID:  accountID,
		APIKey:     sec["api_key"],
		Extras:     sec,
	}
	vectors, usage, ue := cap.Embed(ctx, req.Input, upstream, cred)
	if ue != nil {
		return nil, ue
	}
	resp := &domain.EmbeddingResponse{
		Object: "list",
		Model:  req.Model,
	}
	if usage != nil {
		resp.Usage = *usage
	}
	for i, v := range vectors {
		resp.Data = append(resp.Data, domain.EmbeddingRow{
			Object:    "embedding",
			Index:     i,
			Embedding: v,
		})
	}
	return resp, nil
}

// buildRequest resolves the upstream model + credential and delegates
// to the provider adapter for request shaping.
func (d *ProviderDispatcher) buildRequest(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest) (*http.Request, provider.ChatProvider, error) {
	prov, ok := d.Providers.Lookup(providerID)
	if !ok {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_PROV", "provider not registered").WithMeta("provider", providerID)
	}
	cap := prov.ChatCapability()
	if cap == nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_NOCAP", "provider missing chat capability").WithMeta("provider", providerID)
	}

	// Resolve upstream model and replace the logical id in the request.
	upstream, err := d.Catalog.ResolveUpstreamModel(ctx, req.Model, providerID)
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrModelNotFound, "LLMH_404_001", "model mapping missing").WithCause(err)
	}
	req2 := *req
	req2.Model = upstream

	key, err := d.Pool.ActiveAPIKey(ctx, accountID, "chat")
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_NOKEY", "no active credential").WithCause(err)
	}
	sec, err := d.Vault.Resolve(ctx, key.VaultRef)
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_VAULT", "credential lookup failed").WithCause(err)
	}
	cred := domain.Credential{
		ProviderID: providerID,
		AccountID:  accountID,
		APIKey:     sec["api_key"],
		AccessKey:  sec["access_key"],
		SecretKey:  sec["secret_key"],
		SessionToken: sec["session_token"],
		Extras:     sec,
	}
	httpReq, err := cap.TranslateRequest(ctx, &req2, cred)
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_TR", "translate request").WithCause(err)
	}
	return httpReq, cap, nil
}

// StreamWriter is the subset of http.ResponseWriter we need for SSE.
// Alias of chat.StreamWriter so dispatcher doesn't import the handler.
type StreamWriter interface {
	Write(p []byte) (int, error)
	Flush()
}

// ErrProviderNotFound is returned when the gateway is asked to route
// to an unknown provider id.
var ErrProviderNotFound = errors.New("gateway: provider not found")
