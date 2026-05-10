// Package audit records admin actions to audit.logs. Every mutation
// the operator performs through /api/admin/* should call Record(), so
// in case of dispute / incident we have a paper trail of who changed
// what and when.
//
// The package is deliberately small: a Recorder interface + a Postgres
// implementation. Best-effort writes — a failed audit insert never
// blocks the underlying operation, just logs a warning.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Event is one audit record. Most fields are optional; only Action is
// required (you have to call out *what happened* even if you don't
// know who or against what).
type Event struct {
	ActorType  string         // "admin" / "user" / "system" / "sdk"
	ActorID    int64          // 0 if N/A (e.g. ActorType="admin" with shared token)
	Action     string         // verb_object, e.g. "grant_subscription" / "revoke_lease"
	TargetType string         // "user" / "subscription" / "lease" / "credential" / "platform_service"
	TargetID   string         // id of the target row, stringified
	IP         string         // request RemoteAddr (port stripped)
	UserAgent  string         // request User-Agent
	Result     string         // "ok" / "denied" / "error"
	Payload    map[string]any // small structured context (id changes, before/after diffs, etc.)
}

// Recorder is what handler code uses. Production wire is *PgRecorder;
// tests pass a no-op or a slice-collector.
type Recorder interface {
	Record(ctx context.Context, ev Event)
}

// Nop is a Recorder that does nothing. Useful when audit is disabled
// or in tests that don't care about audit side-effects.
type Nop struct{}

// Record on Nop is a no-op.
func (Nop) Record(_ context.Context, _ Event) {}

// PgRecorder writes Events into audit.logs. Failures get logged but
// never returned — admin actions don't fail just because audit is down.
type PgRecorder struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewPgRecorder builds a Postgres-backed Recorder.
func NewPgRecorder(pool *pgxpool.Pool, logger *slog.Logger) *PgRecorder {
	return &PgRecorder{pool: pool, logger: logger}
}

// Record inserts an audit row. Non-blocking effort — slow audit DB
// shouldn't slow the user-visible request, so callers commonly invoke
// this as `go r.Record(...)`. We accept that and don't add internal
// fan-out; that policy lives at the call site.
func (r *PgRecorder) Record(ctx context.Context, ev Event) {
	const sql = `
INSERT INTO audit.logs
       (actor_type, actor_id, action, target_type, target_id,
        ip, user_agent, result, payload)
VALUES ($1, NULLIF($2,0), $3, NULLIF($4,''), NULLIF($5,''),
        NULLIF($6,'')::inet, NULLIF($7,''), NULLIF($8,''), $9)
`
	var payloadBytes []byte
	if ev.Payload != nil {
		b, err := json.Marshal(ev.Payload)
		if err != nil {
			// A bad payload should still produce an audit row — drop the
			// payload and log the marshal error instead of swallowing
			// the entire event.
			r.logger.WarnContext(ctx, "audit payload marshal failed", "err", err, "action", ev.Action)
		} else {
			payloadBytes = b
		}
	}
	if payloadBytes == nil {
		payloadBytes = []byte("{}")
	}
	ip := stripPort(ev.IP)
	if _, err := r.pool.Exec(ctx, sql,
		ev.ActorType, ev.ActorID, ev.Action,
		ev.TargetType, ev.TargetID,
		ip, ev.UserAgent, ev.Result, payloadBytes,
	); err != nil {
		r.logger.WarnContext(ctx, "audit write failed",
			"err", err, "action", ev.Action,
			"actor_type", ev.ActorType, "target_type", ev.TargetType, "target_id", ev.TargetID)
	}
}

// stripPort returns the host portion of "host:port"; passes empty
// strings through. INET columns reject "1.2.3.4:5678" so admin
// handlers should always use this helper before recording.
func stripPort(addr string) string {
	if addr == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// Row is the read-side projection used by /api/admin/audit-logs.
type Row struct {
	ID         int64
	ActorType  string
	ActorID    *int64
	Action     string
	TargetType string
	TargetID   string
	IP         string
	UserAgent  string
	Result     string
	Payload    json.RawMessage
	CreatedAt  string
}

// Filter narrows the read-side query.
type Filter struct {
	ActorID    int64
	Action     string
	TargetType string
	TargetID   string
	Limit      int
}

// List returns the most-recent audit rows matching the filter.
func (r *PgRecorder) List(ctx context.Context, f Filter) ([]Row, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	where := []string{"1 = 1"}
	args := []any{}
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, clause+itoa(len(args)))
	}
	if f.ActorID > 0 {
		add("actor_id = $", f.ActorID)
	}
	if f.Action != "" {
		add("action = $", f.Action)
	}
	if f.TargetType != "" {
		add("target_type = $", f.TargetType)
	}
	if f.TargetID != "" {
		add("target_id = $", f.TargetID)
	}
	args = append(args, f.Limit)
	limitArg := "$" + itoa(len(args))

	sql := `
SELECT id, actor_type, actor_id, action,
       COALESCE(target_type,''), COALESCE(target_id,''),
       COALESCE(ip::text,''), COALESCE(user_agent,''),
       COALESCE(result,''), COALESCE(payload, '{}'::jsonb),
       to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM audit.logs
WHERE ` + joinAnd(where) + `
ORDER BY id DESC
LIMIT ` + limitArg

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var rec Row
		if err := rows.Scan(
			&rec.ID, &rec.ActorType, &rec.ActorID, &rec.Action,
			&rec.TargetType, &rec.TargetID, &rec.IP, &rec.UserAgent,
			&rec.Result, &rec.Payload, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// itoa / joinAnd are tiny inlined helpers so the package doesn't
// import strconv / strings just for SQL placeholder building.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}
