// Package domain contains the core data types shared across LLMHub
// services. Values here are plain Go structs (no external deps beyond
// stdlib) so they can be embedded anywhere — gateway, scheduler, billing,
// admin — without creating a dependency graph.
package domain

import "time"

// Transport enumerates how a capability speaks to clients / upstreams.
type Transport string

const (
	TransportHTTP      Transport = "http"
	TransportSSE       Transport = "sse"
	TransportWebSocket Transport = "websocket"
	TransportWebRTC    Transport = "webrtc"
)

// BillingUnit is the unit by which a capability meters usage.
type BillingUnit string

const (
	UnitToken  BillingUnit = "token"
	UnitSecond BillingUnit = "second"
	UnitChar   BillingUnit = "char"
	UnitImage  BillingUnit = "image"
	UnitPage   BillingUnit = "page"
	UnitCall   BillingUnit = "call"
)

// Tier of an upstream account in the pool.
type Tier string

const (
	TierT1 Tier = "T1" // enterprise plan
	TierT2 Tier = "T2" // personal paid
	TierT3 Tier = "T3" // new-user bonus
	TierT4 Tier = "T4" // reseller
)

// RiskLevel influences the tier preference used by the scheduler.
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
)

// Usage is the unified usage reported by any capability.
// Only the fields relevant to a capability will be populated.
type Usage struct {
	InputTokens   int     `json:"input_tokens,omitempty"`
	OutputTokens  int     `json:"output_tokens,omitempty"`
	TotalTokens   int     `json:"total_tokens,omitempty"`
	AudioSeconds  float64 `json:"audio_seconds,omitempty"`
	Characters    int     `json:"characters,omitempty"`
	Images        int     `json:"images,omitempty"`
	Pages         int     `json:"pages,omitempty"`
}

// Pricing is the price applied for one unit.
type Pricing struct {
	Unit                BillingUnit
	InputPer1KCentsX100 int64 // price per 1k input units, in 1/100 cent (precision)
	OutputPer1KCentsX100 int64
}

// BillingEstimate is the result of a pre-call estimation used for
// balance freeze. Cents is in 1/100 cent to preserve precision.
type BillingEstimate struct {
	Unit          BillingUnit
	EstimatedUnits float64
	Cents         int64 // amount to freeze in 1/100 cent (i.e. 1 cent = 100)
}

// Credential is the runtime-injected upstream credential set.
// Never persist this struct; it exists only for the lifetime of a call.
type Credential struct {
	ProviderID   string
	AccountID    int64
	APIKey       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	Extras       map[string]string
}

// CallMeta is attached to both logs and successful response envelopes.
type CallMeta struct {
	RequestID     string
	Provider      string
	UpstreamModel string
	AccountID     int64
	BilledCents   int64
	StartedAt     time.Time
	EndedAt       time.Time
	TTFBMs        int
	DurationMs    int
	RetryCount    int
	FallbackChain []string
}

// Result is a generic envelope for capability outputs.
type Result[T any] struct {
	Value T
	Usage Usage
	Meta  CallMeta
}
