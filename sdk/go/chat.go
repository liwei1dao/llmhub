package llmhub

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatRequest is the OpenAI-shaped chat request the SDK accepts. Most
// LLM providers in the platform speak this dialect; vendor-specific
// fields (Volc Ark function-calling extensions, DeepSeek reasoning
// flags) can be passed through Extra.
type ChatRequest struct {
	// Model is a SKU id from the platform's catalog (e.g.
	// "deepseek-chat" / "doubao-pro"). The SDK uses it to fetch a
	// lease and to look up the upstream model name.
	Model       string         `json:"-"`
	Messages    []ChatMessage  `json:"messages"`
	Stream      bool           `json:"stream,omitempty"`
	Temperature *float32       `json:"temperature,omitempty"`
	TopP        *float32       `json:"top_p,omitempty"`
	MaxTokens   *int           `json:"max_tokens,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	User        string         `json:"user,omitempty"`
	// Extra carries vendor-specific fields untouched. Keys are merged
	// into the upstream JSON body alongside the standard fields.
	Extra map[string]any `json:"-"`
}

// ChatMessage is a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatResponse mirrors the OpenAI chat-completions shape. The SDK
// exposes it verbatim — providers may add upstream-specific fields
// inside `choices[*].message.<vendor_extra>`; pull those out via
// json.RawMessage if needed.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatChoice is one completion alternative.
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"` // streaming only
}

// ChatUsage is upstream-reported token usage. The SDK forwards these
// numbers to the platform's /sdk/usage/report (in 1k-token units when
// the SKU's billing_unit is "1k_tokens").
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Chat runs a non-streaming chat completion. The SDK:
//
//  1. Resolves req.Model → SKU lease (cached / refreshed as needed)
//  2. Picks the wire transport based on lease.ProtocolFamily
//  3. Calls the upstream provider directly with the leased credential
//  4. Reports usage back to the platform asynchronously
//
// On non-2xx upstream responses, returns *APIError with Source="upstream"
// (Source="platform" for issue/auth failures).
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil || req.Model == "" {
		return nil, fmt.Errorf("llmhub: ChatRequest.Model is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("llmhub: ChatRequest.Messages is required")
	}
	cctx, cancel := c.ctxOrClosed(ctx)
	defer cancel()

	lease, err := c.leases.get(cctx, req.Model, "")
	if err != nil {
		return nil, err
	}
	t, ok := transportFor(lease.ProtocolFamily)
	if !ok {
		return nil, fmt.Errorf("llmhub: unsupported protocol_family %q for sku %q",
			lease.ProtocolFamily, req.Model)
	}
	start := time.Now()
	resp, ttfb, err := t.invokeChat(cctx, c, lease, req)
	latency := time.Since(start)

	c.maybeInvalidateOnAuthError(req.Model, err)
	c.reportChatOutcome(lease, resp, err, latency, ttfb)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ChatStream runs a streaming chat completion. Caller drains
// Stream.Chunks until it closes; check Stream.Err afterwards.
type Stream struct {
	chunks chan StreamChunk
	err    error
	done   chan struct{}
}

// StreamChunk is one SSE-decoded delta. Usage is non-nil only on the
// terminal frame (vendor-dependent — DeepSeek and Ark both emit it).
type StreamChunk struct {
	Raw   json.RawMessage // the full upstream frame, in case the caller wants vendor-specific fields
	Delta *ChatMessage
	Usage *ChatUsage
	Done  bool
}

// Chunks returns the receive channel; closes when the stream ends.
func (s *Stream) Chunks() <-chan StreamChunk { return s.chunks }

// Err returns the terminal error, or nil if the stream completed
// cleanly. Only valid after Chunks() closes.
func (s *Stream) Err() error {
	<-s.done
	return s.err
}

// ChatStream starts a streaming chat. Sets req.Stream=true regardless
// of input. The returned Stream's Chunks channel is closed when the
// upstream sends [DONE] or the SDK encounters an error.
func (c *Client) ChatStream(ctx context.Context, req *ChatRequest) (*Stream, error) {
	if req == nil || req.Model == "" {
		return nil, fmt.Errorf("llmhub: ChatRequest.Model is required")
	}
	cctx, cancel := c.ctxOrClosed(ctx)

	lease, err := c.leases.get(cctx, req.Model, "")
	if err != nil {
		cancel()
		return nil, err
	}
	t, ok := transportFor(lease.ProtocolFamily)
	if !ok {
		cancel()
		return nil, fmt.Errorf("llmhub: unsupported protocol_family %q", lease.ProtocolFamily)
	}

	streamReq := *req
	streamReq.Stream = true
	stream := &Stream{
		chunks: make(chan StreamChunk, 16),
		done:   make(chan struct{}),
	}
	go func() {
		defer close(stream.chunks)
		defer close(stream.done)
		defer cancel()
		start := time.Now()
		usage, ttfb, err := t.invokeChatStream(cctx, c, lease, &streamReq, stream)
		c.maybeInvalidateOnAuthError(req.Model, err)
		latency := time.Since(start)
		var resp *ChatResponse
		if usage != nil {
			resp = &ChatResponse{Usage: *usage}
		}
		c.reportChatOutcome(lease, resp, err, latency, ttfb)
		stream.err = err
	}()
	return stream, nil
}

// reportChatOutcome computes a usage report from a chat result and
// hands it to the async reporter. We deliberately don't propagate
// reporter failures back to the caller — usage events are soft.
func (c *Client) reportChatOutcome(lease *Lease, resp *ChatResponse, err error, latency, ttfb time.Duration) {
	ev := usageReport{
		LeaseID:   lease.LeaseID,
		Status:    "success",
		LatencyMs: latency.Milliseconds(),
		TTFBMs:    ttfb.Milliseconds(),
	}
	if resp != nil {
		ev.InputUnits = int64(resp.Usage.PromptTokens)
		ev.OutputUnits = int64(resp.Usage.CompletionTokens)
	}
	if err != nil {
		ev.Status = errOutcome(err)
		ev.ErrorCode = errCode(err)
	}
	c.report.enqueue(ev)
}

func (c *Client) maybeInvalidateOnAuthError(sku string, err error) {
	if err == nil {
		return
	}
	ae, ok := err.(*APIError)
	if !ok {
		return
	}
	// 401 from upstream means the leased credential is no longer
	// valid (e.g. platform rotated the underlying upstream key);
	// drop the cached lease so the next call re-issues.
	if ae.Status == http.StatusUnauthorized && ae.Source == "upstream" {
		c.leases.invalidate(sku)
	}
}

// errOutcome maps an SDK error onto the report status enum the
// platform expects. Anything we don't classify falls through to
// "upstream_error".
func errOutcome(err error) string {
	ae, ok := err.(*APIError)
	if !ok {
		return "upstream_error"
	}
	switch ae.Status {
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "auth_failed"
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return "timeout"
	}
	return "upstream_error"
}

func errCode(err error) string {
	if ae, ok := err.(*APIError); ok {
		return ae.Code
	}
	return ""
}

// readAndDiscard drains an http body fully so the underlying
// connection can be reused by net/http's keep-alive logic.
func readAndDiscard(r io.Reader) (int64, error) {
	return io.Copy(io.Discard, r)
}

// ----- transport plumbing -----

// transport is the wire-protocol seam. The SDK ships with a single
// implementation today (openai_compat) that covers DeepSeek + Volc Ark.
// Volc speech / Anthropic / Aliyun NLS ship as additional transports.
type transport interface {
	invokeChat(ctx context.Context, c *Client, lease *Lease, req *ChatRequest) (*ChatResponse, time.Duration, error)
	invokeChatStream(ctx context.Context, c *Client, lease *Lease, req *ChatRequest, out *Stream) (*ChatUsage, time.Duration, error)
}

var transports = map[string]transport{}

func transportFor(family string) (transport, bool) {
	t, ok := transports[family]
	return t, ok
}

// registerTransport is called from the per-protocol files in init().
func registerTransport(family string, t transport) { transports[family] = t }

// ----- helpers shared across transports -----

func mergeBody(req *ChatRequest, model string) ([]byte, error) {
	// We marshal the SDK ChatRequest to JSON, then splice in Extra
	// keys (so callers can pass through vendor-specific fields).
	base, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	m["model"] = model
	for k, v := range req.Extra {
		m[k] = v
	}
	return json.Marshal(m)
}

// makeUpstreamError converts a non-2xx upstream response into APIError.
func makeUpstreamError(status int, body []byte) *APIError {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &env)
	code := env.Error.Code
	if code == "" {
		code = env.Error.Type
	}
	msg := env.Error.Message
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	return &APIError{Status: status, Code: code, Message: msg, Source: "upstream"}
}

// ----- streaming SSE helpers -----

type sseLine struct {
	data []byte
}

func readSSE(rd io.Reader, onChunk func(payload []byte) error) error {
	br := bufio.NewReaderSize(rd, 64*1024)
	var line []byte
	for {
		piece, err := br.ReadBytes('\n')
		if len(piece) > 0 {
			line = append(line, bytes.TrimRight(piece, "\r\n")...)
			// SSE separates events by a blank line; a single line
			// without trailing blank can still be processed as the
			// vendor's encoding sometimes drops trailing blanks.
			if !bytes.HasPrefix(line, []byte("data:")) {
				line = nil
				if err == io.EOF {
					return nil
				}
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			line = nil
			if string(payload) == "[DONE]" {
				return nil
			}
			if err := onChunk(payload); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// silence unused
var _ sseLine
