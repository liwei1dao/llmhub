package repo

import (
	"context"
	"time"
)

// CredentialEvent captures one state-change / probe / upstream-error
// observation. Append-only — used for diagnostics and audit.
type CredentialEvent struct {
	ID           int64
	CredentialID *int64
	BindingID    *int64
	EventType    string
	FromState    string
	ToState      string
	Capability   string
	Reason       string
	CreatedAt    time.Time
}

// EventRepo is the data-access surface for pool.credential_events.
type EventRepo struct{ r *Repo }

// NewEventRepo binds to the shared pool.
func NewEventRepo(r *Repo) *EventRepo { return &EventRepo{r: r} }

// Events is sugar for repo.NewEventRepo.
func (r *Repo) Events() *EventRepo { return NewEventRepo(r) }

// Insert appends a credential event. At least one of CredentialID or
// BindingID must be set (CHECK constraint enforces this at DB level).
func (er *EventRepo) Insert(ctx context.Context, ev CredentialEvent) error {
	const q = `
INSERT INTO pool.credential_events
       (credential_id, binding_id, event_type, from_state, to_state, capability, reason)
VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''))
`
	_, err := er.r.pool.Exec(ctx, q,
		ev.CredentialID, ev.BindingID, ev.EventType,
		ev.FromState, ev.ToState, ev.Capability, ev.Reason,
	)
	return err
}

// ListByCredential returns the latest N events for a credential
// (and any of its bindings — via credential_id JOIN above the
// binding's credential_id is implicit, so we filter by either side).
func (er *EventRepo) ListByCredential(ctx context.Context, credID int64, limit int) ([]CredentialEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
SELECT id, credential_id, binding_id, event_type,
       COALESCE(from_state,''), COALESCE(to_state,''),
       COALESCE(capability,''), COALESCE(reason,''),
       created_at
FROM pool.credential_events
WHERE credential_id = $1
   OR binding_id IN (SELECT id FROM pool.credential_services WHERE credential_id = $1)
ORDER BY created_at DESC
LIMIT $2
`
	rows, err := er.r.pool.Query(ctx, q, credID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CredentialEvent
	for rows.Next() {
		var ev CredentialEvent
		if err := rows.Scan(
			&ev.ID, &ev.CredentialID, &ev.BindingID, &ev.EventType,
			&ev.FromState, &ev.ToState,
			&ev.Capability, &ev.Reason,
			&ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
