// Package provider defines the upstream provider abstraction and a
// generic registry. Concrete provider adapters live under
// internal/provider/<id>/ and register themselves via init().
package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/llmhub/llmhub/internal/domain"
)

// Config is everything a provider adapter needs to instantiate.
// Loaded from configs/providers/<id>.yaml (plus optional DB overrides).
type Config struct {
	ID                    string                 `yaml:"id"`
	DisplayName           string                 `yaml:"display_name"`
	BaseURL               string                 `yaml:"base_url"`
	Auth                  AuthConfig             `yaml:"auth"`
	ProtocolFamily        string                 `yaml:"protocol_family"`
	SupportedCapabilities []string               `yaml:"supported_capabilities"`
	Models                map[string][]ModelSpec `yaml:"models"`
	ErrorMapping          map[string]string      `yaml:"error_mapping"`
	QuotaQuery            *QuotaQuery            `yaml:"quota_query,omitempty"`
	Probe                 map[string]ProbeSpec   `yaml:"probe,omitempty"`
}

// AuthConfig describes how to authenticate to the upstream.
type AuthConfig struct {
	Mode   string `yaml:"mode"`   // bearer / ak_sk / signed_token
	Header string `yaml:"header"` // header name for bearer
}

// ModelSpec is a capability-keyed model mapping.
type ModelSpec struct {
	LogicalID string                 `yaml:"logical_id"`
	Upstream  string                 `yaml:"upstream"`
	Pricing   map[string]any         `yaml:"pricing"`
	Extras    map[string]any         `yaml:"extras,omitempty"`
}

// QuotaQuery describes how to read upstream remaining quota.
type QuotaQuery struct {
	Type     string `yaml:"type"`     // "api" / "manual"
	Endpoint string `yaml:"endpoint,omitempty"`
}

// ProbeSpec is used by pool capability probing to issue a minimal
// test request against a new account.
type ProbeSpec struct {
	Model      string `yaml:"model,omitempty"`
	Voice      string `yaml:"voice,omitempty"`
	TestPrompt string `yaml:"test_prompt,omitempty"`
	TestText   string `yaml:"test_text,omitempty"`
}

// Provider is the top-level adapter interface. Each capability is
// exposed via its own accessor; returning nil means this provider does
// not support that capability.
type Provider interface {
	ID() string
	Meta() domain.ProviderMeta

	ChatCapability() ChatProvider
	EmbeddingCapability() EmbeddingProvider
	ASRCapability() ASRProvider
	TTSCapability() TTSProvider
	TranslateCapability() TranslateProvider
	ImageCapability() ImageProvider
	VLMCapability() VLMProvider
}

// ChatProvider adapts a vendor's chat API to the platform's unified shape.
type ChatProvider interface {
	TranslateRequest(ctx context.Context, req *domain.ChatRequest, cred domain.Credential) (*http.Request, error)
	ParseResponse(ctx context.Context, resp *http.Response) (*domain.ChatResponse, *domain.Usage, error)
	TranslateStreamChunk(raw []byte) ([]byte, *domain.Usage, error)
	MapError(statusCode int, body []byte) *domain.UnifiedError
}

// EmbeddingProvider adapts a vendor's embedding API.
type EmbeddingProvider interface {
	Embed(ctx context.Context, inputs []string, model string, cred domain.Credential) ([][]float32, *domain.Usage, *domain.UnifiedError)
}

// ASRProvider adapts batch + streaming speech recognition.
type ASRProvider interface {
	TranscribeBatch(ctx context.Context, req *domain.ASRRequest, cred domain.Credential) (*domain.ASRResponse, *domain.Usage, *domain.UnifiedError)
	StreamEndpoint(ctx context.Context, req *domain.ASRStreamRequest, cred domain.Credential) (*domain.UpstreamStreamEndpoint, *domain.UnifiedError)
}

// TTSProvider adapts a vendor's speech synthesis API.
type TTSProvider interface {
	Synthesize(ctx context.Context, req *domain.TTSRequest, cred domain.Credential) (*domain.TTSResponse, *domain.UnifiedError)
	SynthesizeStream(ctx context.Context, req *domain.TTSRequest, cred domain.Credential) (io.ReadCloser, *domain.UnifiedError)
}

// TranslateProvider adapts a text translation vendor.
type TranslateProvider interface {
	Translate(ctx context.Context, req *domain.TranslateRequest, cred domain.Credential) (*domain.TranslateResponse, *domain.UnifiedError)
}

// ImageProvider adapts a text-to-image vendor. V2 adds edits/variations.
type ImageProvider interface {
	Generate(ctx context.Context, prompt string, size string, n int, cred domain.Credential) ([]string, *domain.UnifiedError)
}

// VLMProvider adapts a visual language model vendor.
// VLM reuses the chat request shape with image_url content parts.
type VLMProvider interface {
	ChatProvider
}

// Factory constructs a Provider from config.
type Factory func(cfg Config) (Provider, error)

// Providers is the global registry of provider factories.
var Providers = newRegistry[Factory]()

// registry is a generic thread-safe lookup table.
type registry[T any] struct {
	mu    sync.RWMutex
	items map[string]T
}

func newRegistry[T any]() *registry[T] {
	return &registry[T]{items: make(map[string]T)}
}

// Register registers a value under id.
func (r *registry[T]) Register(id string, v T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; ok {
		panic(fmt.Sprintf("provider %q already registered", id))
	}
	r.items[id] = v
}

// Get fetches a value by id.
func (r *registry[T]) Get(id string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	return v, ok
}

// All returns a snapshot of registered ids.
func (r *registry[T]) All() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.items))
	for k := range r.items {
		ids = append(ids, k)
	}
	return ids
}
