package llmhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Lease is the platform's response to /sdk/credentials/issue. It
// contains the *real* upstream credential and is the most sensitive
// object in the SDK: never log it, never persist it, zero on expiry.
type Lease struct {
	LeaseID        string            `json:"lease_id"`
	IssuedAt       time.Time         `json:"issued_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	Vendor         string            `json:"vendor"`
	VendorProduct  string            `json:"vendor_product"`
	Capability     string            `json:"capability"`
	UpstreamModel  string            `json:"upstream_model"`
	Endpoint       string            `json:"endpoint"`
	ProtocolFamily string            `json:"protocol_family"`
	AuthPayload    map[string]string `json:"auth_payload"`
}

// expired returns true if the lease is at or past its TTL minus the
// configured lead time.
func (l *Lease) expired(leadTime time.Duration) bool {
	if l == nil {
		return true
	}
	return time.Now().Add(leadTime).After(l.ExpiresAt)
}

// zero overwrites every byte of the auth_payload values so the
// underlying map (when GC'd) doesn't leave the upstream key floating
// in heap memory. Doesn't fully prevent memory inspection — Go strings
// are immutable + can be copied during GC — but it removes the most
// obvious traces. For stronger guarantees, the SDK would need cgo +
// mlock + manual memory management; deliberately out of scope here.
func (l *Lease) zero() {
	if l == nil {
		return
	}
	for k := range l.AuthPayload {
		// Replace with all-zero string of equal length so any
		// dangling references see scrubbed data.
		l.AuthPayload[k] = strings.Repeat("\x00", len(l.AuthPayload[k]))
		delete(l.AuthPayload, k)
	}
}

// leaseCache holds active leases keyed by sku_id. Refreshing happens
// lazily on next call; we don't run a background goroutine to keep
// the SDK process footprint minimal.
type leaseCache struct {
	c    *Client
	mu   sync.Mutex
	data map[string]*Lease
}

func newLeaseCache(c *Client) *leaseCache {
	return &leaseCache{c: c, data: make(map[string]*Lease)}
}

// get returns a fresh lease for sku, refreshing from the platform if
// the cached one is missing / expired.
func (lc *leaseCache) get(ctx context.Context, sku, fingerprint string) (*Lease, error) {
	lc.mu.Lock()
	cur, ok := lc.data[sku]
	if ok && !cur.expired(lc.c.cfg.LeaseLeadTime) {
		lc.mu.Unlock()
		return cur, nil
	}
	// Drop the old one; we'll request fresh below outside the lock so
	// concurrent SKU lookups don't all stall on the same call.
	if ok {
		cur.zero()
		delete(lc.data, sku)
	}
	lc.mu.Unlock()

	fresh, err := lc.c.issue(ctx, sku, fingerprint)
	if err != nil {
		return nil, err
	}
	lc.mu.Lock()
	lc.data[sku] = fresh
	lc.mu.Unlock()
	return fresh, nil
}

// invalidate drops the cached lease for sku. Call on a 401 from
// upstream — the platform may have rotated the underlying credential.
func (lc *leaseCache) invalidate(sku string) {
	lc.mu.Lock()
	if cur, ok := lc.data[sku]; ok {
		cur.zero()
		delete(lc.data, sku)
	}
	lc.mu.Unlock()
}

// purgeAll wipes every cached lease. Called from Client.Close.
func (lc *leaseCache) purgeAll() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for k, l := range lc.data {
		l.zero()
		delete(lc.data, k)
	}
}

// issue calls POST /sdk/credentials/issue.
func (c *Client) issue(ctx context.Context, sku, fingerprint string) (*Lease, error) {
	body := map[string]string{"sku_id": sku}
	if fingerprint != "" {
		body["client_fingerprint"] = fingerprint
	}
	buf, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/sdk/credentials/issue", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Cache-Control", "no-store")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llmhub: issue request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodePlatformError(resp)
	}
	var lease Lease
	if err := json.NewDecoder(resp.Body).Decode(&lease); err != nil {
		return nil, fmt.Errorf("llmhub: parse lease: %w", err)
	}
	return &lease, nil
}

func decodePlatformError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
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
		msg = string(body)
	}
	return &APIError{
		Status:  resp.StatusCode,
		Code:    code,
		Message: msg,
		Source:  "platform",
	}
}
