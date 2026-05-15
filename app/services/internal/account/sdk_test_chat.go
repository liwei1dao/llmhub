package account

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/llmhub/llmhub/internal/sdkapi"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// 用户控制台「在线测试」走的路径：
//   POST /api/user/sdk-test/chat
//
// 这条路径不像 SDK 那样需要 plaintext api_key —— 浏览器已经携带
// session cookie，后端从中拿到 user_id，配合用户在下拉里选的
// api_key_id（仅做用量归因 / 健康反馈）就能复用 sdkapi.IssueLease
// 的全部业务校验链。控制台不暴露上游 auth_payload，直接 SSR 端
// 把请求转发到 lease.Endpoint。

type sdkTestChatReq struct {
	SKUID       string                   `json:"sku_id"`
	APIKeyID    int64                    `json:"api_key_id,omitempty"`
	Messages    []map[string]any         `json:"messages"`
	Temperature *float32                 `json:"temperature,omitempty"`
	MaxTokens   *int                     `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Extra       map[string]any           `json:"extra,omitempty"` // vendor-specific passthrough
}

type leaseMetaView struct {
	LeaseID        string    `json:"lease_id"`
	Vendor         string    `json:"vendor"`
	VendorProduct  string    `json:"vendor_product"`
	Capability     string    `json:"capability"`
	UpstreamModel  string    `json:"upstream_model"`
	Endpoint       string    `json:"endpoint"`
	ProtocolFamily string    `json:"protocol_family"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// handleSDKTestChat is the user-facing "试一下" handler.
//
// Auth:    requireUser middleware sets userIDFrom(ctx).
// Body:    sdkTestChatReq (no plaintext required).
// Flow:    pick / validate api_key_id → sdkapi.IssueLease → upstream
//          /chat/completions → sdkapi.IngestUsage.
func (s *Server) handleSDKTestChat(w http.ResponseWriter, r *http.Request) {
	if s.sdk == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "sdk_not_wired", "sdk api is not wired in this binary")
		return
	}
	uid := userIDFrom(r.Context())
	if uid == 0 {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "session required")
		return
	}

	var req sdkTestChatReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.SKUID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "sku_id is required")
		return
	}
	if len(req.Messages) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "messages is required")
		return
	}

	// Resolve api_key_id: must belong to user + active. If client didn't
	// pick one, default to the first active key. We need *some* api_key_id
	// because pool.leases.api_key_id is NOT NULL.
	apiKeyID, kerr := s.resolveAPIKey(r.Context(), uid, req.APIKeyID)
	if kerr != nil {
		httpx.Error(w, kerr.Status, kerr.Code, kerr.Message)
		return
	}

	// Issue the lease via the shared sdkapi core.
	lease, ierr := s.sdk.IssueLease(r.Context(), sdkapi.IssueParams{
		UserID:   uid,
		APIKeyID: apiKeyID,
		SKUID:    req.SKUID,
	})
	if ierr != nil {
		httpx.Error(w, ierr.Status, ierr.Code, ierr.Message)
		return
	}

	meta := leaseMetaView{
		LeaseID:        lease.LeaseID,
		Vendor:         lease.Vendor,
		VendorProduct:  lease.VendorProduct,
		Capability:     lease.Capability,
		UpstreamModel:  lease.UpstreamModel,
		Endpoint:       lease.Endpoint,
		ProtocolFamily: lease.ProtocolFamily,
		IssuedAt:       lease.IssuedAt,
		ExpiresAt:      lease.ExpiresAt,
	}

	// Pick the bearer the upstream expects. Same precedence as the SDK
	// transport so we don't drift between the two paths.
	token := pickUpstreamToken(lease.AuthPayload)
	if token == "" {
		s.logger.ErrorContext(r.Context(), "sdk-test: lease has no usable bearer", "lease", lease.LeaseID)
		httpx.Error(w, http.StatusBadGateway, "bad_lease", "lease has no bearer in auth_payload")
		return
	}
	model := lease.UpstreamModel
	if model == "" {
		model = lease.AuthPayload["app_id"]
	}
	if model == "" {
		httpx.Error(w, http.StatusBadGateway, "bad_lease", "lease has no upstream_model")
		return
	}

	body := map[string]any{
		"model":    model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	url := strings.TrimRight(lease.Endpoint, "/") + "/chat/completions"
	start := time.Now()

	// ─── streaming branch ───
	if req.Stream {
		body["stream"] = true
		bodyJSON, _ := json.Marshal(body)
		req2, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(bodyJSON))
		if err != nil {
			s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, time.Since(start), 0, "upstream_error", "build_request")
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req2)
		if err != nil {
			s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, time.Since(start), 0, "upstream_error", "network")
			httpx.Error(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		ttfb := time.Since(start)
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(resp.Body)
			s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, time.Since(start), ttfb, outcomeFromStatus(resp.StatusCode), fmt.Sprintf("%d", resp.StatusCode))
			httpx.JSON(w, http.StatusBadGateway, map[string]any{
				"lease": meta,
				"error": map[string]any{"code": "upstream_error", "status": resp.StatusCode, "message": string(b)},
			})
			return
		}

		// SSE pipe with two custom events bracketing it: `lease` and `summary`.
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		writeSSE := func(event, data string) {
			if event != "" {
				_, _ = fmt.Fprintf(w, "event: %s\n", event)
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		}
		metaJSON, _ := json.Marshal(meta)
		writeSSE("lease", string(metaJSON))

		// Walk SSE lines; pass them through verbatim and sniff `usage` for the summary.
		br := bufio.NewReaderSize(resp.Body, 64*1024)
		var lastUsage map[string]int
		for {
			line, err := br.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = w.Write(line)
				if flusher != nil {
					flusher.Flush()
				}
				t := strings.TrimSpace(strings.TrimRight(string(line), "\r\n"))
				if strings.HasPrefix(t, "data:") {
					payload := strings.TrimSpace(t[len("data:"):])
					if payload != "" && payload != "[DONE]" {
						var frame struct {
							Usage *struct {
								PromptTokens     int `json:"prompt_tokens"`
								CompletionTokens int `json:"completion_tokens"`
								TotalTokens      int `json:"total_tokens"`
							} `json:"usage"`
						}
						if json.Unmarshal([]byte(payload), &frame) == nil && frame.Usage != nil {
							lastUsage = map[string]int{
								"prompt_tokens":     frame.Usage.PromptTokens,
								"completion_tokens": frame.Usage.CompletionTokens,
								"total_tokens":      frame.Usage.TotalTokens,
							}
						}
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
		latency := time.Since(start)

		summary := map[string]any{
			"usage":      lastUsage,
			"latency_ms": latency.Milliseconds(),
			"ttfb_ms":    ttfb.Milliseconds(),
		}
		summaryJSON, _ := json.Marshal(summary)
		writeSSE("summary", string(summaryJSON))

		// Report usage now that we have token counts.
		var in, out int64
		if lastUsage != nil {
			in = int64(lastUsage["prompt_tokens"])
			out = int64(lastUsage["completion_tokens"])
		}
		s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, in, out, latency, ttfb, "success", "")
		return
	}

	// ─── non-streaming branch ───
	bodyJSON, _ := json.Marshal(body)
	req2, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, time.Since(start), 0, "upstream_error", "build_request")
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req2)
	if err != nil {
		s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, time.Since(start), 0, "upstream_error", "network")
		httpx.Error(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	ttfb := time.Since(start)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, latency, ttfb, outcomeFromStatus(resp.StatusCode), fmt.Sprintf("%d", resp.StatusCode))
		httpx.JSON(w, http.StatusBadGateway, map[string]any{
			"lease": meta,
			"error": map[string]any{"code": "upstream_error", "status": resp.StatusCode, "message": string(respBody)},
		})
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, 0, 0, latency, ttfb, "upstream_error", "parse")
		httpx.JSON(w, http.StatusBadGateway, map[string]any{
			"lease": meta,
			"error": map[string]any{"code": "parse_error", "message": "upstream returned non-JSON", "raw": string(respBody)},
		})
		return
	}

	// Extract usage tokens for reporting + UI.
	var in, out int64
	if u, ok := parsed["usage"].(map[string]any); ok {
		in = int64(numFromAny(u["prompt_tokens"]))
		out = int64(numFromAny(u["completion_tokens"]))
	}
	s.reportUsageBestEffort(r.Context(), uid, lease.LeaseID, in, out, latency, ttfb, "success", "")

	httpx.JSON(w, http.StatusOK, map[string]any{
		"lease":      meta,
		"response":   parsed,
		"latency_ms": latency.Milliseconds(),
		"ttfb_ms":    ttfb.Milliseconds(),
	})
}

// resolveAPIKey ensures the api_key_id (if supplied) belongs to user
// and is active; otherwise picks the first active key for that user.
type apiKeyErr struct {
	Status  int
	Code    string
	Message string
}

func (s *Server) resolveAPIKey(ctx context.Context, userID, picked int64) (int64, *apiKeyErr) {
	keys, err := s.iam.ListAPIKeys(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "sdk-test: list api keys", "err", err, "user", userID)
		return 0, &apiKeyErr{Status: http.StatusInternalServerError, Code: "internal_error", Message: "api key lookup failed"}
	}
	if picked != 0 {
		for _, k := range keys {
			if k.ID == picked {
				if k.Status != "active" {
					return 0, &apiKeyErr{Status: http.StatusForbidden, Code: "api_key_revoked", Message: "selected api key is not active"}
				}
				return k.ID, nil
			}
		}
		return 0, &apiKeyErr{Status: http.StatusForbidden, Code: "api_key_not_found", Message: "api key does not belong to the current account"}
	}
	for _, k := range keys {
		if k.Status == "active" {
			return k.ID, nil
		}
	}
	return 0, &apiKeyErr{Status: http.StatusForbidden, Code: "no_api_key", Message: "create an api key first"}
}

func (s *Server) reportUsageBestEffort(
	ctx context.Context, userID int64, leaseID string,
	in, out int64, latency, ttfb time.Duration,
	status, errCode string,
) {
	if s.sdk == nil || leaseID == "" {
		return
	}
	req := &sdkapi.UsageReportRequest{
		LeaseID:     leaseID,
		InputUnits:  in,
		OutputUnits: out,
		Status:      status,
		ErrorCode:   errCode,
		LatencyMs:   latency.Milliseconds(),
		TTFBMs:      ttfb.Milliseconds(),
	}
	if _, _, _, ok := s.sdk.IngestUsage(ctx, userID, req); !ok {
		// IngestUsage already logs; nothing else useful here.
		_ = errors.New("usage report skipped")
	}
}

// pickUpstreamToken: Volc Ark publishes either `app_token` or `api_key`
// depending on which credential generation tool the operator used.
// Order matches the SDK's transport_openai_compat.pickToken.
func pickUpstreamToken(payload map[string]string) string {
	for _, k := range []string{"app_token", "api_key"} {
		if v := strings.TrimSpace(payload[k]); v != "" {
			return v
		}
	}
	return ""
}

func outcomeFromStatus(status int) string {
	switch status {
	case 429:
		return "rate_limited"
	case 401, 403:
		return "auth_failed"
	case 408, 504:
		return "timeout"
	}
	return "upstream_error"
}

func numFromAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}
