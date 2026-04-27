package repo

import (
	"context"
	"time"
)

// UsageBucket is one day of usage for a single user/capability.
type UsageBucket struct {
	Day             time.Time
	CapabilityID    string
	Calls           int64
	SuccessCalls    int64
	TokensIn        int64
	TokensOut       int64
	AudioSeconds    float64
	Characters      int64
	CostRetailCents float64
}

// UserUsageSeries returns daily usage rows for a user inside [from, to].
// Rows are sorted oldest-first; a contiguous date range is the caller's
// responsibility (the chart UI fills holes with zeros).
func (r *Repo) UserUsageSeries(ctx context.Context, userID int64, from, to time.Time) ([]UsageBucket, error) {
	const sql = `
SELECT date_trunc('day', ts) AS day,
       capability_id,
       COUNT(*) AS calls,
       COUNT(*) FILTER (WHERE status = 'success') AS success_calls,
       COALESCE(SUM(tokens_in), 0)         AS tokens_in,
       COALESCE(SUM(tokens_out), 0)        AS tokens_out,
       COALESCE(SUM(audio_seconds), 0)     AS audio_seconds,
       COALESCE(SUM(characters), 0)        AS characters,
       COALESCE(SUM(cost_retail_cents), 0) AS cost_retail_cents
FROM metering.call_logs
WHERE user_id = $1
  AND ts >= $2 AND ts < $3
GROUP BY 1, 2
ORDER BY 1 ASC, 2 ASC
`
	rows, err := r.pool.Query(ctx, sql, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageBucket
	for rows.Next() {
		var b UsageBucket
		if err := rows.Scan(
			&b.Day, &b.CapabilityID,
			&b.Calls, &b.SuccessCalls, &b.TokensIn, &b.TokensOut,
			&b.AudioSeconds, &b.Characters, &b.CostRetailCents,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
