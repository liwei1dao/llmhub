package repo

import "context"

// SetSupportedCapabilities overwrites the account's
// supported_capabilities column. Used by the probe flow to record the
// capabilities that actually returned 2xx from the upstream.
func (r *Repo) SetSupportedCapabilities(ctx context.Context, accountID int64, caps []string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE pool.accounts SET supported_capabilities = $1, updated_at = NOW() WHERE id = $2`,
		caps, accountID,
	)
	return err
}

// RecordProbeEvent appends a probe-run entry to the account_events log.
func (r *Repo) RecordProbeEvent(ctx context.Context, accountID int64, result string, reason string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO pool.account_events (account_id, event_type, reason, metadata)
         VALUES ($1, 'capability_probe', $2, jsonb_build_object('result', $3))`,
		accountID, reason, result,
	)
	return err
}
