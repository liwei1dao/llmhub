package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/platform/id"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/pkg/errcode"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Deps bundles the external services the chat handler needs.
// Using small interfaces keeps the handler testable with fakes and
// lets us swap the in-process scheduler for a remote client.
type Deps struct {
	Logger    *slog.Logger
	Scheduler SchedulerClient
	Billing   BillingClient
	Auth      AuthResolver
	Provider  ProviderInvoker
	Catalog   *catalog.Service
	Publisher EventPublisher
}

// EventPublisher emits llmhub.call.completed events. nil is allowed —
// the handler simply skips publishing when no publisher is wired.
type EventPublisher interface {
	PublishCallCompleted(ctx context.Context, payload []byte)
}

// SchedulerClient is satisfied by both *scheduler.Service (in-process)
// and *scheduler/client.Client (remote HTTP).
type SchedulerClient interface {
	Pick(ctx context.Context, req scheduler.PickRequest) (*scheduler.PickResult, error)
	Report(ctx context.Context, accountID int64, r scheduler.ReportResult) error
}

// BillingClient is the subset of billing we need from the chat handler.
type BillingClient interface {
	Freeze(ctx context.Context, requestID string, userID, cents int64) error
	Settle(ctx context.Context, requestID string, cents int64) error
	Release(ctx context.Context, requestID string) error
}

// AuthResolver converts an incoming Bearer token into a user id.
type AuthResolver interface {
	AuthenticateAPIKey(ctx context.Context, plaintext string) (userID int64, apiKeyID int64, err error)
}

// ProviderInvoker runs the actual upstream call once the scheduler has
// picked an account. For non-streaming requests callers use Invoke;
// for streaming they use InvokeStream and provide the ResponseWriter
// shim to flush SSE frames as they arrive.
type ProviderInvoker interface {
	Invoke(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest) (*domain.ChatResponse, error)
	InvokeStream(ctx context.Context, providerID string, accountID int64, req *domain.ChatRequest, w StreamWriter) (*domain.Usage, error)
}

// StreamWriter is the subset of http.ResponseWriter used for SSE.
type StreamWriter interface {
	Write(p []byte) (int, error)
	Flush()
}

// NewHandler builds the http.Handler for POST /v1/chat/completions.
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

	var req domain.ChatRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Model == "" || len(req.Messages) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "model and messages are required")
		return
	}

	// Look up pricing once so we can freeze an accurate amount and
	// settle correctly later. Unknown-model pricing -> reject early.
	retailPricing, err := d.Catalog.Pricing(ctx, req.Model, "", "chat", "retail")
	if err != nil {
		// Fallback: pricing-by-provider (nil provider in catalog is optional).
		d.Logger.WarnContext(ctx, "pricing lookup failed", "model", req.Model, "err", err)
	}

	holdCents := EstimateFreeze(retailPricing, inputChars(&req), maxOutputTokens(&req))
	if err := d.Billing.Freeze(ctx, requestID, userID, holdCents); err != nil {
		d.Logger.WarnContext(ctx, "billing freeze failed", "err", err, "user_id", userID)
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
		CapabilityID: "chat",
		ProviderID:   inferProvider(req.Model),
		ModelID:      req.Model,
		RiskLevel:    domain.RiskMedium,
		SessionKey:   req.User,
	})
	if err != nil {
		writeDomainErr(w, err)
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if req.Stream {
		d.runStreaming(callCtx, w, pick, &req, requestID, retailPricing)
		settled = true // settle happened inside runStreaming
		_ = d.Scheduler.Report(ctx, pick.AccountID, scheduler.ReportSuccess)
		return
	}

	resp, err := d.Provider.Invoke(callCtx, pick.ProviderID, pick.AccountID, &req)
	if err != nil {
		_ = d.Scheduler.Report(ctx, pick.AccountID, reportFromError(err))
		writeDomainErr(w, err)
		return
	}

	cost := CostFromUsage(retailPricing, resp.Usage)
	if err := d.Billing.Settle(ctx, requestID, cost); err != nil {
		d.Logger.ErrorContext(ctx, "billing settle failed", "err", err, "request_id", requestID)
	}
	settled = true
	_ = d.Scheduler.Report(ctx, pick.AccountID, scheduler.ReportSuccess)

	publishCallCompleted(ctx, d, completedPayload{
		RequestID:    requestID,
		UserID:       userID,
		Capability:   "chat",
		ModelID:      req.Model,
		ProviderID:   pick.ProviderID,
		AccountID:    pick.AccountID,
		Status:       "success",
		Usage:        resp.Usage,
		BilledCents:  cost,
	})

	out := struct {
		*domain.ChatResponse
		Meta map[string]any `json:"llmhub_meta"`
	}{
		ChatResponse: resp,
		Meta: map[string]any{
			"request_id":   requestID,
			"provider":     pick.ProviderID,
			"account_id":   pick.AccountID,
			"billed_cents": cost,
		},
	}
	httpx.JSON(w, http.StatusOK, out)
}

// runStreaming handles the SSE branch and manages settle/release
// inside so the caller can treat it as a single flow.
func (d Deps) runStreaming(ctx context.Context, w http.ResponseWriter, pick *scheduler.PickResult, req *domain.ChatRequest, requestID string, pricing *pricingAlias) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		_ = d.Billing.Release(ctx, requestID)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "streaming not supported by response writer")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// Send a preliminary comment frame so proxies don't buffer us.
	_, _ = w.Write([]byte(": ok\n\n"))
	flusher.Flush()

	sw := streamFlusher{w: w, f: flusher}

	usage, err := d.Provider.InvokeStream(ctx, pick.ProviderID, pick.AccountID, req, sw)
	if err != nil {
		// Write a final SSE error frame so SDKs can surface it.
		b, _ := json.Marshal(map[string]any{"error": map[string]any{"message": err.Error()}})
		_, _ = sw.Write([]byte("data: "))
		_, _ = sw.Write(b)
		_, _ = sw.Write([]byte("\n\n"))
		sw.Flush()
		_ = d.Billing.Release(ctx, requestID)
		_ = d.Scheduler.Report(ctx, pick.AccountID, reportFromError(err))
		return
	}

	var actual domain.Usage
	if usage != nil {
		actual = *usage
	}
	cost := CostFromUsage(pricing, actual)
	if err := d.Billing.Settle(ctx, requestID, cost); err != nil {
		d.Logger.ErrorContext(ctx, "billing settle failed (stream)", "err", err, "request_id", requestID)
	}
	publishCallCompleted(ctx, d, completedPayload{
		RequestID:   requestID,
		Capability:  "chat",
		ModelID:     req.Model,
		ProviderID:  pick.ProviderID,
		AccountID:   pick.AccountID,
		Status:      "success",
		Usage:       actual,
		BilledCents: cost,
	})
	// Trailing metadata frame for clients that want billing info inline.
	meta, _ := json.Marshal(map[string]any{
		"llmhub_meta": map[string]any{
			"request_id":   requestID,
			"provider":     pick.ProviderID,
			"account_id":   pick.AccountID,
			"billed_cents": cost,
		},
	})
	_, _ = sw.Write([]byte("data: "))
	_, _ = sw.Write(meta)
	_, _ = sw.Write([]byte("\n\n"))
	sw.Flush()
}

type streamFlusher struct {
	w http.ResponseWriter
	f http.Flusher
}

func (s streamFlusher) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s streamFlusher) Flush()                      { s.f.Flush() }

// ---------- helpers ----------

func parseBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

// inferProvider is a placeholder until the scheduler is capability-aware
// of model → provider mappings. For now we route every chat request to
// volc; the scheduler itself will pick which volc account.
func inferProvider(_ string) string { return "volc" }

func inputChars(req *domain.ChatRequest) int {
	n := 0
	for _, m := range req.Messages {
		if s, ok := m.Content.(string); ok {
			n += len(s)
		}
	}
	return n
}

func maxOutputTokens(req *domain.ChatRequest) int {
	if req.MaxTokens != nil {
		return *req.MaxTokens
	}
	return 2048
}

func writeDomainErr(w http.ResponseWriter, err error) {
	ue, ok := err.(*domain.UnifiedError)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, string(domain.ErrInternal), err.Error())
		return
	}
	status := errcode.HTTPStatus(ue.Kind)
	httpx.JSON(w, status, map[string]any{
		"error": map[string]any{
			"type":    string(ue.Kind),
			"code":    ue.Code,
			"message": ue.Message,
		},
	})
}

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
