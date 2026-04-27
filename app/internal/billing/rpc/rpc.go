// Package rpc defines the wire shape of the inter-service billing RPC.
//
// The payloads mirror the proto messages defined under proto/billing/v1
// so the HTTP/JSON transport used today can be swapped for real gRPC
// in M8 without any caller changes. Keep the JSON tags stable.
package rpc

// FreezeRequest pre-authorizes `cents` against a user's balance.
type FreezeRequest struct {
	RequestID string `json:"request_id"`
	UserID    int64  `json:"user_id"`
	Cents     int64  `json:"cents"`
}

// FreezeResponse is the freeze outcome.
type FreezeResponse struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// SettleRequest finalizes a hold with the real consumed cents.
type SettleRequest struct {
	RequestID   string `json:"request_id"`
	ActualCents int64  `json:"actual_cents"`
}

// ReleaseRequest returns a held amount to the user without charging.
type ReleaseRequest struct {
	RequestID string `json:"request_id"`
}

// Empty is the canonical "ok, nothing to tell you" response.
type Empty struct{}
