// Package embedding hosts the /v1/embeddings HTTP handler.
//
// Embeddings are simpler than chat — no streaming, no tool calls, no
// retry-with-fallback semantics yet. The handler still goes through
// the full freeze/pick/settle pipeline so usage rolls into the same
// metering pipeline as chat.
package embedding

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/llmhub/llmhub/internal/catalog"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/platform/id"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/pkg/errcode"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Deps mirrors chat.Deps but for the embedding capability.
type Deps struct {
	Logger    *slog.Logger
	Scheduler SchedulerClient
	Billing   BillingClient
	Auth      AuthResolver
	Provider  Invoker
	Catalog   *catalog.Service
	Publisher EventPublisher
}

// SchedulerClient mirrors chat's interface so the same scheduler
// service / client double-implements both ports.
type SchedulerClient interface {
	Pick(ctx context.Context, req scheduler.PickRequest) (*scheduler.PickResult, error)
	Report(ctx context.Context, accountID int64, r scheduler.ReportResult) error
}

// BillingClient is the freeze/settle/release surface.
type BillingClient interface {
	Freeze(ctx context.Context, requestID string, userID, cents int64) error
	Settle(ctx context.Context, requestID string, cents int64) error
	Release(ctx context.Context, requestID string) error
}

// AuthResolver authenticates the bearer token.
type AuthResolver interface {
	AuthenticateAPIKey(ctx context.Context, plaintext string) (userID int64, apiKeyID int64, err error)
}

// Invoker runs the upstream embedding call.
type Invoker interface {
	Invoke(ctx context.Context, providerID string, accountID int64, req *domain.EmbeddingRequest) (*domain.EmbeddingResponse, error)
}

// EventPublisher emits llmhub.call.completed events. nil is a no-op.
type EventPublisher interface {
	PublishCallCompleted(ctx context.Context, payload []byte)
}

// NewHandler builds the http.Handler for POST /v1/embeddings.
func NewHandler(d Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { d.handle(w, r) })
}

func (d Deps) handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := id.Prefixed("req")

	bearer := parseBearer(r)
	if bearer == "" {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	userID, _, err := d.Auth.AuthenticateAPIKey(ctx, bearer)
	if err != nil || userID == 0 {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
		return
	}

	var req domain.EmbeddingRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Model == "" || len(req.Input) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "model and input are required")
		return
	}

	pricing, err := d.Catalog.Pricing(ctx, req.Model, "", "embedding", "retail")
	if err != nil {
		d.Logger.WarnContext(ctx, "embedding pricing lookup failed", "model", req.Model, "err", err)
	}
	chars := totalChars(req.Input)
	holdCents := estimateFreeze(pricing, chars)
	if err := d.Billing.Freeze(ctx, requestID, userID, holdCents); err != nil {
		httpx.Error(w, http.StatusPaymentRequired, "insufficient_balance", err.Error())
		return
	}
	settled := false
	defer func() {
		if !settled {
			_ = d.Billing.Release(ctx, requestID)
		}
	}()

	pick, err := d.Scheduler.Pick(ctx, scheduler.PickRequest{
		RequestID:    requestID,
		UserID:       userID,
		CapabilityID: "embedding",
		ProviderID:   inferProvider(req.Model),
		ModelID:      req.Model,
		RiskLevel:    domain.RiskMedium,
	})
	if err != nil {
		writeDomainErr(w, err)
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := d.Provider.Invoke(callCtx, pick.ProviderID, pick.AccountID, &req)
	if err != nil {
		_ = d.Scheduler.Report(ctx, pick.AccountID, reportFromError(err))
		writeDomainErr(w, err)
		return
	}

	cost := costFromUsage(pricing, resp.Usage)
	if err := d.Billing.Settle(ctx, requestID, cost); err != nil {
		d.Logger.ErrorContext(ctx, "embedding billing settle failed", "err", err)
	}
	settled = true
	_ = d.Scheduler.Report(ctx, pick.AccountID, scheduler.ReportSuccess)

	publishCompleted(ctx, d, completedPayload{
		RequestID:   requestID,
		UserID:      userID,
		ModelID:     req.Model,
		ProviderID:  pick.ProviderID,
		AccountID:   pick.AccountID,
		Status:      "success",
		Usage:       resp.Usage,
		BilledCents: cost,
	})

	out := struct {
		*domain.EmbeddingResponse
		Meta map[string]any `json:"llmhub_meta"`
	}{
		EmbeddingResponse: resp,
		Meta: map[string]any{
			"request_id":   requestID,
			"provider":     pick.ProviderID,
			"account_id":   pick.AccountID,
			"billed_cents": cost,
		},
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ---------- helpers ----------

func parseBearer(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func decodeRequest(w http.ResponseWriter, r *http.Request, v *domain.EmbeddingRequest) bool {
	const maxBody = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)

	// Accept both "input": "string" and "input": ["a", "b"].
	var raw struct {
		Model          string          `json:"model"`
		Input          json.RawMessage `json:"input"`
		EncodingFormat string          `json:"encoding_format,omitempty"`
		Dimensions     int             `json:"dimensions,omitempty"`
		User           string          `json:"user,omitempty"`
	}
	if err := dec.Decode(&raw); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return false
	}
	v.Model = raw.Model
	v.EncodingFormat = raw.EncodingFormat
	v.Dimensions = raw.Dimensions
	v.User = raw.User
	if len(raw.Input) == 0 {
		return true
	}
	if raw.Input[0] == '[' {
		_ = json.Unmarshal(raw.Input, &v.Input)
	} else {
		var s string
		_ = json.Unmarshal(raw.Input, &s)
		v.Input = []string{s}
	}
	return true
}

func totalChars(in []string) int {
	n := 0
	for _, s := range in {
		n += len(s)
	}
	return n
}

func estimateFreeze(p *catalogrepo.Pricing, chars int) int64 {
	tokens := chars / 3
	var cents float64
	if p != nil {
		cents = float64(tokens) / 1000.0 * p.InputPer1KCents
	}
	cents *= 2
	if cents < 50 {
		cents = 50
	}
	return int64(ceil(cents))
}

func costFromUsage(p *catalogrepo.Pricing, u domain.Usage) int64 {
	if p == nil {
		if u.InputTokens > 0 {
			return 1
		}
		return 0
	}
	cents := float64(u.InputTokens) / 1000.0 * p.InputPer1KCents
	if cents < 1 && u.InputTokens > 0 {
		cents = 1
	}
	return int64(ceil(cents))
}

// ceil rounds up without bringing in math just for one call.
func ceil(v float64) float64 {
	if v == float64(int64(v)) {
		return v
	}
	if v >= 0 {
		return float64(int64(v)) + 1
	}
	return float64(int64(v))
}

func inferProvider(_ string) string { return "deepseek" }

func reportFromError(err error) scheduler.ReportResult {
	ue, ok := err.(*domain.UnifiedError)
	if !ok {
		return scheduler.ReportUpstreamErr
	}
	switch ue.Kind {
	case domain.ErrRateLimited:
		return scheduler.ReportRateLimited
	case domain.ErrUnauthorized:
		return scheduler.ReportAuthFailed
	case domain.ErrUpstreamTimeout:
		return scheduler.ReportTimeout
	}
	return scheduler.ReportUpstreamErr
}

func writeDomainErr(w http.ResponseWriter, err error) {
	ue, ok := err.(*domain.UnifiedError)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, string(domain.ErrInternal), err.Error())
		return
	}
	httpx.JSON(w, errcode.HTTPStatus(ue.Kind), map[string]any{
		"error": map[string]any{
			"type":    string(ue.Kind),
			"code":    ue.Code,
			"message": ue.Message,
		},
	})
}
