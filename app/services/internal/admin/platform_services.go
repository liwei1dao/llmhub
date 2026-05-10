package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// ────────────────────────────────────────────────────────────
// Platform service (SKU) admin endpoints — catalog.platform_services
// + catalog.platform_pricing (current price inlined into the list view).
// ────────────────────────────────────────────────────────────

// PlatformServiceView is the JSON payload returned by list/get. The
// "current_*_cents" fields are the latest pricing row for the SKU
// (NULL if no pricing has been set yet).
type PlatformServiceView struct {
	ID                 string   `json:"id"`
	CategoryID         string   `json:"category_id"`
	DisplayName        string   `json:"display_name"`
	Description        string   `json:"description,omitempty"`
	VendorProductID    string   `json:"vendor_product_id"`
	Capability         string   `json:"capability"`
	UpstreamModel      string   `json:"upstream_model,omitempty"`
	BillingUnit        string   `json:"billing_unit"`
	ContextWindow      *int     `json:"context_window,omitempty"`
	MaxOutputTokens    *int     `json:"max_output_tokens,omitempty"`
	IsPublic           bool     `json:"is_public"`
	SortOrder          int      `json:"sort_order"`
	Tags               []string `json:"tags,omitempty"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	CurrentInputCents  *float64 `json:"current_input_cents,omitempty"`
	CurrentOutputCents *float64 `json:"current_output_cents,omitempty"`
	CurrentImageCents  *float64 `json:"current_image_cents,omitempty"`
}

// listPlatformServices handles GET /api/admin/platform-services.
//
// Filters: category_id, vendor_product_id, status, q (display_name LIKE).
// Joins the latest platform_pricing row by effective_from DESC so the
// admin SKU table can show current price without an extra round-trip.
func (s *Server) listPlatformServices(w http.ResponseWriter, r *http.Request) {
	const sql = `
SELECT
  s.id, s.category_id, s.display_name, COALESCE(s.description,''),
  s.vendor_product_id, s.capability, COALESCE(s.upstream_model,''),
  s.billing_unit, s.context_window, s.max_output_tokens,
  s.is_public, s.sort_order, COALESCE(s.tags, '{}'::text[]),
  s.status, s.created_at, s.updated_at,
  p.input_per_unit_cents, p.output_per_unit_cents, p.image_per_unit_cents
FROM catalog.platform_services s
LEFT JOIN LATERAL (
  SELECT input_per_unit_cents, output_per_unit_cents, image_per_unit_cents
  FROM catalog.platform_pricing
  WHERE platform_service_id = s.id
    AND (effective_until IS NULL OR effective_until > NOW())
  ORDER BY effective_from DESC
  LIMIT 1
) p ON TRUE
WHERE ($1 = '' OR s.category_id = $1)
  AND ($2 = '' OR s.vendor_product_id = $2)
  AND ($3 = '' OR s.status = $3)
  AND ($4 = '' OR s.display_name ILIKE '%'||$4||'%' OR s.id ILIKE '%'||$4||'%')
ORDER BY s.category_id ASC, s.sort_order ASC, s.id ASC
LIMIT $5
`
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 200
	}
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	pp := s.pool.Repo().Pool() // bare pgx pool
	rows, err := pp.Query(r.Context(), sql,
		r.URL.Query().Get("category_id"),
		r.URL.Query().Get("vendor_product_id"),
		r.URL.Query().Get("status"),
		r.URL.Query().Get("q"),
		limit,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	defer rows.Close()

	out := make([]PlatformServiceView, 0)
	for rows.Next() {
		var v PlatformServiceView
		var ctxWin, maxOut *int
		var inP, outP, imgP *float64
		if err := rows.Scan(
			&v.ID, &v.CategoryID, &v.DisplayName, &v.Description,
			&v.VendorProductID, &v.Capability, &v.UpstreamModel,
			&v.BillingUnit, &ctxWin, &maxOut,
			&v.IsPublic, &v.SortOrder, &v.Tags,
			&v.Status, scanTimestamp(&v.CreatedAt), scanTimestamp(&v.UpdatedAt),
			&inP, &outP, &imgP,
		); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		v.ContextWindow = ctxWin
		v.MaxOutputTokens = maxOut
		v.CurrentInputCents = inP
		v.CurrentOutputCents = outP
		v.CurrentImageCents = imgP
		out = append(out, v)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// scanTimestamp returns a sql.Scanner that formats a timestamp as
// RFC3339 directly into a string field — saves the per-row
// allocation of a time.Time on the hot list path.
type tsScanner struct{ dst *string }

func scanTimestamp(dst *string) any { return &tsScanner{dst: dst} }

func (s *tsScanner) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s.dst = ""
	case string:
		*s.dst = v
	default:
		// pgx returns time.Time for timestamptz
		if t, ok := src.(interface{ Format(string) string }); ok {
			*s.dst = t.Format("2006-01-02T15:04:05Z07:00")
			return nil
		}
		*s.dst = ""
	}
	return nil
}

// createPlatformServiceReq is the body for POST. The handler validates
// each field against the static catalog dictionary before insert and,
// if any pricing fields are set, writes a row to platform_pricing.
type createPlatformServiceReq struct {
	ID                string   `json:"id"`
	CategoryID        string   `json:"category_id"`
	DisplayName       string   `json:"display_name"`
	Description       string   `json:"description"`
	VendorProductID   string   `json:"vendor_product_id"`
	Capability        string   `json:"capability"`
	UpstreamModel     string   `json:"upstream_model"`
	BillingUnit       string   `json:"billing_unit"`
	ContextWindow     *int     `json:"context_window,omitempty"`
	MaxOutputTokens   *int     `json:"max_output_tokens,omitempty"`
	IsPublic          *bool    `json:"is_public,omitempty"`
	SortOrder         int      `json:"sort_order"`
	Tags              []string `json:"tags"`
	InputPerUnitCents *float64 `json:"input_per_unit_cents,omitempty"`
	OutputPerUnitCents *float64 `json:"output_per_unit_cents,omitempty"`
	ImagePerUnitCents *float64 `json:"image_per_unit_cents,omitempty"`
}

// createPlatformService handles POST /api/admin/platform-services.
//
// Static-catalog invariants enforced here:
//   - id is unique (DB unique key)
//   - category_id ∈ catalog.Categories
//   - vendor_product_id ∈ catalog.Products
//   - capability ∈ Products[vendor_product_id].AllowedCapabilities
//
// SKU + initial pricing are written in a single transaction.
func (s *Server) createPlatformService(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	var req createPlatformServiceReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.ID == "" || req.DisplayName == "" || req.BillingUnit == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"id, display_name, billing_unit are required")
		return
	}
	if _, ok := catalog.LookupCategory(req.CategoryID); !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"unknown category_id: "+req.CategoryID)
		return
	}
	if _, ok := catalog.LookupProduct(req.VendorProductID); !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"unknown vendor_product_id: "+req.VendorProductID)
		return
	}
	if !catalog.ProductAllowsCapability(req.VendorProductID, req.Capability) {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"capability "+req.Capability+" not allowed by product "+req.VendorProductID)
		return
	}
	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	pp := s.pool.Repo().Pool()
	err := pgx.BeginFunc(r.Context(), pp, func(tx pgx.Tx) error {
		const insSKU = `
INSERT INTO catalog.platform_services
  (id, category_id, display_name, description, vendor_product_id, capability,
   upstream_model, billing_unit, context_window, max_output_tokens,
   is_public, sort_order, tags)
VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,
        NULLIF($7,''),$8,$9,$10,
        $11,
        COALESCE(NULLIF($12,0), 100),
        COALESCE($13::text[], '{}'::text[]))
`
		if _, err := tx.Exec(r.Context(), insSKU,
			req.ID, req.CategoryID, req.DisplayName, req.Description,
			req.VendorProductID, req.Capability,
			req.UpstreamModel, req.BillingUnit, req.ContextWindow, req.MaxOutputTokens,
			isPublic, req.SortOrder, req.Tags,
		); err != nil {
			return err
		}
		if req.InputPerUnitCents != nil || req.OutputPerUnitCents != nil || req.ImagePerUnitCents != nil {
			const insPrice = `
INSERT INTO catalog.platform_pricing
  (platform_service_id, input_per_unit_cents, output_per_unit_cents, image_per_unit_cents, notes)
VALUES ($1, $2, $3, $4, 'initial')
`
			if _, err := tx.Exec(r.Context(), insPrice,
				req.ID, req.InputPerUnitCents, req.OutputPerUnitCents, req.ImagePerUnitCents,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"id":"` + req.ID + `"}`))
}

// patchPlatformServiceReq mutates status / display fields. Pricing is
// updated by inserting a new platform_pricing row (history-preserving),
// not via PATCH on the SKU itself — kept for a separate endpoint.
type patchPlatformServiceReq struct {
	Status      string   `json:"status,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	IsPublic    *bool    `json:"is_public,omitempty"`
	SortOrder   *int     `json:"sort_order,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// patchPlatformService handles PATCH /api/admin/platform-services/{id}.
func (s *Server) patchPlatformService(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id := chi.URLParam(r, "id")
	var req patchPlatformServiceReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	// Build a small dynamic UPDATE — keeping it inline since the field
	// set is bounded and won't grow.
	const sql = `
UPDATE catalog.platform_services SET
  status       = COALESCE(NULLIF($2,''),       status),
  display_name = COALESCE(NULLIF($3,''),       display_name),
  is_public    = COALESCE($4::boolean,         is_public),
  sort_order   = COALESCE($5::int,             sort_order),
  tags         = COALESCE($6::text[],          tags),
  updated_at   = NOW()
WHERE id = $1
`
	pp := s.pool.Repo().Pool()
	tag, err := pp.Exec(r.Context(), sql,
		id, req.Status, req.DisplayName, req.IsPublic, req.SortOrder, req.Tags,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "platform_service not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── pricing edit ─────────────────────────

// updatePricingReq is the body for POST /api/admin/platform-services/{id}/pricing.
// Same shape as the initial pricing inside CreatePlatformServiceReq —
// fields are optional, only those present get applied.
type updatePricingReq struct {
	InputPerUnitCents  *float64 `json:"input_per_unit_cents,omitempty"`
	OutputPerUnitCents *float64 `json:"output_per_unit_cents,omitempty"`
	ImagePerUnitCents  *float64 `json:"image_per_unit_cents,omitempty"`
	Notes              string   `json:"notes,omitempty"`
}

// updatePlatformServicePricing appends a new pricing row that becomes
// the currently-effective price (effective_from=NOW, effective_until=NULL).
// We close out the previous active row by stamping its effective_until
// in the same tx — the LATERAL lookup in the SKU list query already
// orders by effective_from DESC, so this preserves history.
func (s *Server) updatePlatformServicePricing(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "sku id is required")
		return
	}
	var req updatePricingReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.InputPerUnitCents == nil && req.OutputPerUnitCents == nil && req.ImagePerUnitCents == nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"at least one of input/output/image_per_unit_cents is required")
		return
	}

	pp := s.pool.Repo().Pool()
	tx, err := pp.Begin(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// Confirm SKU exists; surfaces 404 cleanly without a foreign-key error.
	var exists bool
	if err := tx.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM catalog.platform_services WHERE id = $1)`, id,
	).Scan(&exists); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !exists {
		httpx.Error(w, http.StatusNotFound, "not_found", "platform_service not found")
		return
	}

	// Close out any currently-effective price row.
	if _, err := tx.Exec(r.Context(), `
UPDATE catalog.platform_pricing
SET effective_until = NOW()
WHERE platform_service_id = $1
  AND effective_until IS NULL
`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Append the new row.
	if _, err := tx.Exec(r.Context(), `
INSERT INTO catalog.platform_pricing
       (platform_service_id, input_per_unit_cents, output_per_unit_cents, image_per_unit_cents, notes)
VALUES ($1, $2, $3, $4, NULLIF($5,''))
`, id, req.InputPerUnitCents, req.OutputPerUnitCents, req.ImagePerUnitCents, req.Notes,
	); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.recordAdmin(r, "update_pricing", "platform_service", id, "ok", map[string]any{
		"input_per_unit_cents":  req.InputPerUnitCents,
		"output_per_unit_cents": req.OutputPerUnitCents,
		"image_per_unit_cents":  req.ImagePerUnitCents,
		"notes":                 req.Notes,
	})
	w.WriteHeader(http.StatusNoContent)
}

// silence unused imports in case any of the helpers are pruned
var _ = errors.Is
var _ context.Context
