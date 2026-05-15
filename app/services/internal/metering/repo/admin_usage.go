package repo

import (
	"context"
	"time"
)

// UsageTotals 是 admin "用户用量" 视图里那一排 KPI 数字。
type UsageTotals struct {
	Calls             int64
	SuccessCalls      int64
	TokensIn          int64
	TokensOut         int64
	CostRetailCents   float64
	UniqueSKUs        int64
	AvgLatencyMs      float64 // 仅 success 算入
	AvgTTFBMs         float64
}

// SKUUsageBucket 是按 platform_service_id 聚合的一行 —— 用户用量分布。
type SKUUsageBucket struct {
	SKUID           string
	VendorID        string
	ProductID       string
	Calls           int64
	SuccessCalls    int64
	TokensIn        int64
	TokensOut       int64
	CostRetailCents float64
	LastUsedAt      time.Time
}

// StatusBucket 是按 status 聚合的一行 —— 失败率诊断。
type StatusBucket struct {
	Status string
	Count  int64
}

// DailyUsageBucket 是单日聚合（不再按 capability / SKU 拆），方便画
// "本月调用量 / 消费走势" 这类总体趋势图。
type DailyUsageBucket struct {
	Day             time.Time
	Calls           int64
	SuccessCalls    int64
	TokensIn        int64
	TokensOut       int64
	CostRetailCents float64
}

// RecentCallRow 是 admin "近期调用" 列表里的一行，按时间倒序。
// 我们不暴露 prompt 内容 —— platform 本来就没有 —— 但带上 vendor / sku /
// status / 耗时 / token，足够运营定位异常用户。
type RecentCallRow struct {
	Timestamp  time.Time
	RequestID  string
	SKUID      string
	VendorID   string
	ProductID  string
	Status     string
	ErrorCode  string
	DurationMs int64
	TTFBMs     int64
	TokensIn   int64
	TokensOut  int64
}

// UserUsageTotals 聚合 [from, to) 窗口内的全部 call_logs。仅一行。
func (r *Repo) UserUsageTotals(ctx context.Context, userID int64, from, to time.Time) (UsageTotals, error) {
	const sql = `
SELECT COUNT(*)                                                     AS calls,
       COUNT(*) FILTER (WHERE status = 'success')                   AS success_calls,
       COALESCE(SUM(tokens_in), 0)                                  AS tokens_in,
       COALESCE(SUM(tokens_out), 0)                                 AS tokens_out,
       COALESCE(SUM(cost_retail_cents), 0)                          AS cost_retail_cents,
       COALESCE(COUNT(DISTINCT platform_service_id), 0)             AS unique_skus,
       COALESCE(AVG(duration_ms) FILTER (WHERE status = 'success'), 0) AS avg_latency_ms,
       COALESCE(AVG(ttfb_ms)     FILTER (WHERE status = 'success' AND ttfb_ms IS NOT NULL), 0) AS avg_ttfb_ms
FROM metering.call_logs
WHERE user_id = $1
  AND ts >= $2 AND ts < $3
`
	var t UsageTotals
	err := r.pool.QueryRow(ctx, sql, userID, from, to).Scan(
		&t.Calls, &t.SuccessCalls, &t.TokensIn, &t.TokensOut,
		&t.CostRetailCents, &t.UniqueSKUs, &t.AvgLatencyMs, &t.AvgTTFBMs,
	)
	if err != nil {
		return UsageTotals{}, err
	}
	return t, nil
}

// UserUsageBySKU 按 (sku, vendor, product) 拆分，sort by calls DESC。
// 把"没归属到具体 SKU"的兜底行（platform_service_id 为 NULL）排在最后，
// 让运营一眼看到"这个用户主要在调哪几条 SKU"。
func (r *Repo) UserUsageBySKU(ctx context.Context, userID int64, from, to time.Time) ([]SKUUsageBucket, error) {
	const sql = `
SELECT COALESCE(platform_service_id, '')      AS sku_id,
       COALESCE(vendor_id, '')                AS vendor_id,
       COALESCE(product_id, '')               AS product_id,
       COUNT(*)                               AS calls,
       COUNT(*) FILTER (WHERE status = 'success') AS success_calls,
       COALESCE(SUM(tokens_in), 0)            AS tokens_in,
       COALESCE(SUM(tokens_out), 0)           AS tokens_out,
       COALESCE(SUM(cost_retail_cents), 0)    AS cost_retail_cents,
       MAX(ts)                                AS last_used_at
FROM metering.call_logs
WHERE user_id = $1
  AND ts >= $2 AND ts < $3
GROUP BY 1, 2, 3
ORDER BY calls DESC
LIMIT 50
`
	rows, err := r.pool.Query(ctx, sql, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SKUUsageBucket
	for rows.Next() {
		var b SKUUsageBucket
		if err := rows.Scan(
			&b.SKUID, &b.VendorID, &b.ProductID,
			&b.Calls, &b.SuccessCalls, &b.TokensIn, &b.TokensOut,
			&b.CostRetailCents, &b.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UserUsageByStatus 拆 status —— success / upstream_error / rate_limited
// / auth_failed / timeout —— 配合 totals 算失败率分布。
func (r *Repo) UserUsageByStatus(ctx context.Context, userID int64, from, to time.Time) ([]StatusBucket, error) {
	const sql = `
SELECT status, COUNT(*) AS count
FROM metering.call_logs
WHERE user_id = $1
  AND ts >= $2 AND ts < $3
GROUP BY 1
ORDER BY 2 DESC
`
	rows, err := r.pool.Query(ctx, sql, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatusBucket
	for rows.Next() {
		var b StatusBucket
		if err := rows.Scan(&b.Status, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UserDailySeries 给图表用：每天一行（不拆 SKU），from..to 内的连续序列。
// 行数 = 实际有数据的天数；前端补零。
func (r *Repo) UserDailySeries(ctx context.Context, userID int64, from, to time.Time) ([]DailyUsageBucket, error) {
	const sql = `
SELECT date_trunc('day', ts) AS day,
       COUNT(*) AS calls,
       COUNT(*) FILTER (WHERE status = 'success') AS success_calls,
       COALESCE(SUM(tokens_in), 0)                AS tokens_in,
       COALESCE(SUM(tokens_out), 0)               AS tokens_out,
       COALESCE(SUM(cost_retail_cents), 0)        AS cost_retail_cents
FROM metering.call_logs
WHERE user_id = $1
  AND ts >= $2 AND ts < $3
GROUP BY 1
ORDER BY 1 ASC
`
	rows, err := r.pool.Query(ctx, sql, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyUsageBucket
	for rows.Next() {
		var b DailyUsageBucket
		if err := rows.Scan(
			&b.Day, &b.Calls, &b.SuccessCalls,
			&b.TokensIn, &b.TokensOut, &b.CostRetailCents,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UserRecentCalls 最近 N 条 call_logs（默认 50，上限 200）。
func (r *Repo) UserRecentCalls(ctx context.Context, userID int64, limit int) ([]RecentCallRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	const sql = `
SELECT ts, COALESCE(request_id, ''),
       COALESCE(platform_service_id, ''),
       COALESCE(vendor_id, ''),
       COALESCE(product_id, ''),
       status,
       COALESCE(error_code, ''),
       COALESCE(duration_ms, 0),
       COALESCE(ttfb_ms, 0),
       COALESCE(tokens_in, 0),
       COALESCE(tokens_out, 0)
FROM metering.call_logs
WHERE user_id = $1
ORDER BY ts DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, sql, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentCallRow
	for rows.Next() {
		var c RecentCallRow
		if err := rows.Scan(
			&c.Timestamp, &c.RequestID, &c.SKUID, &c.VendorID, &c.ProductID,
			&c.Status, &c.ErrorCode, &c.DurationMs, &c.TTFBMs,
			&c.TokensIn, &c.TokensOut,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
