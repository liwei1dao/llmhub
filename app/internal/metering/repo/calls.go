package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/llmhub/llmhub/internal/events"
)

// InsertCallLog persists one call.completed event into metering.call_logs.
// The event id is derived from request_id when shaped like a ULID/UUID;
// otherwise we mint a fresh UUID so the (id, ts) primary key is stable.
func (r *Repo) InsertCallLog(ctx context.Context, ev events.CallCompleted) error {
	id := uuid.New()
	ts := ev.StartedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	const sql = `
INSERT INTO metering.call_logs (
    id, ts, user_id, api_key_id, request_id,
    capability_id, model_id, provider_id, pool_account_id,
    status, error_code, duration_ms,
    tokens_in, tokens_out, audio_seconds, characters,
    cost_retail_cents
) VALUES ($1, $2, $3, NULLIF($4, 0), $5,
          $6, $7, $8, $9,
          $10, NULLIF($11, ''), $12,
          $13, $14, $15, $16,
          $17)
`
	_, err := r.pool.Exec(ctx, sql,
		id, ts, ev.UserID, ev.APIKeyID, ev.RequestID,
		ev.CapabilityID, ev.ModelID, ev.ProviderID, ev.AccountID,
		ev.Status, ev.ErrorCode, ev.DurationMs,
		ev.InputTokens, ev.OutputTokens, ev.AudioSeconds, ev.Characters,
		ev.BilledCents,
	)
	return err
}
