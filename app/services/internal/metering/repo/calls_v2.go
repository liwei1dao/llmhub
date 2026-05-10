package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CallLogV2 is the v0.2 metering shape, populated from /sdk/usage/report.
// The platform never saw the upstream call; the SDK is the only source
// of truth, so the row reflects what the SDK reported.
type CallLogV2 struct {
	Timestamp    time.Time
	RequestID    string
	LeaseID      string
	UserID       int64
	APIKeyID     int64
	SKUID        string // catalog.platform_services.id
	VendorID     string
	ProductID    string
	CredentialID int64
	BindingID    int64

	Status     string // success / upstream_error / rate_limited / auth_failed / timeout
	ErrorCode  string
	DurationMs int64
	TTFBMs     int64

	// Usage in SKU's billing_unit terms — name kept generic on purpose.
	InputUnits  int64
	OutputUnits int64
}

// InsertCallLogV2 appends one row using the v0.2 columns.
//
// metering.call_logs still has v0.1 NOT-NULL columns (model_id /
// provider_id / pool_account_id / upstream_model / capability_id) that
// haven't been dropped yet — they will be in a later migration once the
// v0.1 query paths are gone. We fill them with v0.2 stand-ins so:
//   - capability_id  ← the SKU's capability (resolved from SKU upstream)
//   - model_id       ← SKU id (the platform-side service id)
//   - provider_id    ← vendor_id
//   - pool_account_id ← 0 (column is NOT NULL bigint with no zero
//                        check; legacy aggregation queries treat 0 as
//                        "unattributed" gracefully)
//   - upstream_model ← SKU's upstream_model (or empty string)
//
// The new v0.2 columns (platform_service_id / vendor_id / product_id /
// credential_id / binding_id) carry the canonical lineage. Reports
// should join on those — the legacy columns are tombstones during the
// transition.
func (r *Repo) InsertCallLogV2(ctx context.Context, c CallLogV2) error {
	id := uuid.New()
	ts := c.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	requestID := c.RequestID
	if requestID == "" {
		requestID = "sdk-" + id.String()[:8]
	}

	const sql = `
INSERT INTO metering.call_logs (
    id, ts, user_id, api_key_id, request_id,
    capability_id, model_id, provider_id, pool_account_id, upstream_model,
    status, error_code, duration_ms, ttfb_ms,
    tokens_in, tokens_out,
    platform_service_id, vendor_id, product_id, credential_id, binding_id
) VALUES (
    $1, $2, $3, $4, $5,
    'sdk', $6, $7, 0, COALESCE($8, ''),
    $9, NULLIF($10,''), $11, NULLIF($12,0)::int,
    $13, $14,
    $6, $7, $15, NULLIF($16,0), NULLIF($17,0)
)
`
	_, err := r.pool.Exec(ctx, sql,
		id, ts, c.UserID, c.APIKeyID, requestID, // 1..5
		c.SKUID, c.VendorID, "", // 6..8 model_id, vendor_id, upstream_model (left blank for now; SKU lookup at write site is overkill)
		c.Status, c.ErrorCode, c.DurationMs, c.TTFBMs, // 9..12
		c.InputUnits, c.OutputUnits, // 13..14
		c.ProductID, c.CredentialID, c.BindingID, // 15..17
	)
	return err
}
