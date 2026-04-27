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

// EmbeddingAdapter implements provider.EmbeddingProvider against any
// upstream whose /v1/embeddings endpoint follows the OpenAI dialect.
type EmbeddingAdapter struct {
	ProviderTag string
	BaseURL     string
	AuthHeader  string
	HTTP        *http.Client
}

// upstream payload mirrors OpenAI exactly so we can pass it through.
type embedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResp struct {
	Object string                 `json:"object"`
	Data   []domain.EmbeddingRow  `json:"data"`
	Model  string                 `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed performs the upstream call and returns vectors + usage.
func (a *EmbeddingAdapter) Embed(ctx context.Context, inputs []string, model string, cred domain.Credential) ([][]float32, *domain.Usage, *domain.UnifiedError) {
	if len(inputs) == 0 {
		return nil, nil, domain.NewError(domain.ErrInvalidRequest, "LLMH_400_EMBED_EMPTY", "input is empty")
	}
	body, err := json.Marshal(embedReq{Model: model, Input: inputs})
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_EMBED_MARSHAL", err.Error())
	}
	url := strings.TrimRight(a.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrInternal, "LLMH_500_EMBED_REQ", err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	authHeader := a.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}
	if cred.APIKey != "" {
		req.Header.Set(authHeader, "Bearer "+cred.APIKey)
	}
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, domain.NewError(domain.ErrUpstreamError, "LLMH_502_EMBED_DIAL", err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, nil, a.mapErr(resp.StatusCode, raw)
	}

	var out embedResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, domain.NewError(domain.ErrUpstreamError, "LLMH_502_EMBED_PARSE", err.Error())
	}
	vectors := make([][]float32, 0, len(out.Data))
	for _, d := range out.Data {
		vectors = append(vectors, d.Embedding)
	}
	usage := &domain.Usage{
		InputTokens: out.Usage.PromptTokens,
		TotalTokens: out.Usage.TotalTokens,
	}
	return vectors, usage, nil
}

func (a *EmbeddingAdapter) mapErr(statusCode int, body []byte) *domain.UnifiedError {
	tag := a.ProviderTag
	if tag == "" {
		tag = "UPSTREAM"
	}
	switch {
	case statusCode == http.StatusTooManyRequests:
		return domain.NewError(domain.ErrRateLimited, fmt.Sprintf("LLMH_429_EMBED_%s", tag), string(body))
	case statusCode == http.StatusUnauthorized:
		return domain.NewError(domain.ErrUnauthorized, fmt.Sprintf("LLMH_401_EMBED_%s", tag), string(body))
	case statusCode == http.StatusPaymentRequired:
		return domain.NewError(domain.ErrInsufficientBalance, fmt.Sprintf("LLMH_402_EMBED_%s", tag), string(body))
	}
	return domain.NewError(domain.ErrUpstreamError, fmt.Sprintf("LLMH_%d_EMBED_%s", statusCode, tag), string(body))
}
