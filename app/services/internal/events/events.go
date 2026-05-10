// Package events declares the inter-service event payloads exchanged
// over NATS. Subjects are versioned; payload field tags are stable so
// schema evolution stays backwards-compatible.
package events

import "time"

// SubjectCallCompleted is published by the gateway after every chat /
// embedding / asr / tts call (success or failure).
const SubjectCallCompleted = "llmhub.call.completed.v1"

// CallCompleted carries the minimum metering payload. Sensitive
// fields (prompt text, completion text) are deliberately omitted; the
// gateway never publishes user content over the bus.
type CallCompleted struct {
	RequestID    string    `json:"request_id"`
	UserID       int64     `json:"user_id"`
	APIKeyID     int64     `json:"api_key_id,omitempty"`
	CapabilityID string    `json:"capability_id"`
	ModelID      string    `json:"model_id"`
	ProviderID   string    `json:"provider_id"`
	AccountID    int64     `json:"account_id"`
	Status       string    `json:"status"` // success / failed
	ErrorCode    string    `json:"error_code,omitempty"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	AudioSeconds float64   `json:"audio_seconds,omitempty"`
	Characters   int       `json:"characters,omitempty"`
	BilledCents  int64     `json:"billed_cents"`
	DurationMs   int       `json:"duration_ms"`
	StartedAt    time.Time `json:"started_at"`
}
