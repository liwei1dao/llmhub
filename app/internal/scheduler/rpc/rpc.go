// Package rpc declares the wire shape of the inter-service scheduler
// RPC. JSON tags mirror proto/scheduler/v1/scheduler.proto so the
// HTTP/JSON transport used today swaps cleanly to gRPC in a future
// milestone without touching call sites.
package rpc

// PickRequest asks the scheduler for an upstream account.
type PickRequest struct {
	RequestID         string  `json:"request_id"`
	UserID            int64   `json:"user_id"`
	CapabilityID      string  `json:"capability_id"`
	ProviderID        string  `json:"provider_id,omitempty"`
	ModelID           string  `json:"model_id"`
	VoiceID           string  `json:"voice_id,omitempty"`
	EstimatedUnits    int     `json:"estimated_units,omitempty"`
	RiskLevel         int     `json:"risk_level"`
	SessionKey        string  `json:"session_key,omitempty"`
	ExcludeAccountIDs []int64 `json:"exclude_account_ids,omitempty"`
}

// PickResponse is the scheduler's selection.
type PickResponse struct {
	AccountID  int64  `json:"account_id"`
	ProviderID string `json:"provider_id"`
	Tier       string `json:"tier"`
	PickToken  string `json:"pick_token"`
}

// ReportRequest tells the scheduler the outcome of a call so it can
// adjust account health.
type ReportRequest struct {
	RequestID string `json:"request_id"`
	AccountID int64  `json:"account_id"`
	Result    string `json:"result"` // success / upstream_error / rate_limited / auth_failed / timeout
}

// Empty is the canonical "ok" response.
type Empty struct{}
