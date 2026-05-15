package account

import (
	"net/http"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// handleServiceCatalog returns the user-facing service catalog used by
// the 服务列表 page (console). The response packages three pieces the
// frontend needs to render the "可开通服务" modal:
//
//   - categories: the active 平台业务大类 (currently only "llm/大模型")
//   - vendors:    the active 厂商 (currently only "volc/火山引擎")
//   - capabilities: 子分类，按 category_id 分组（"chat" → 文本大模型）
//   - services:   all public, active SKUs with the vendor / product /
//                 capability metadata + current retail pricing inlined
//
// The session middleware on /api/user/* still gates the endpoint — we
// don't want anonymous scraping of the SKU table even though everything
// here is "marketing-public".
func (s *Server) handleServiceCatalog(w http.ResponseWriter, r *http.Request) {
	if s.catalog == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "catalog service not wired")
		return
	}
	skus, err := s.catalog.ListPublicSKUs(r.Context())
	if err != nil {
		s.logger.ErrorContext(r.Context(), "user catalog list", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	services := make([]map[string]any, 0, len(skus))
	for _, sku := range skus {
		row := map[string]any{
			"id":                sku.ID,
			"display_name":      sku.DisplayName,
			"category_id":       sku.CategoryID,
			"vendor_product_id": sku.VendorProductID,
			"capability":        sku.Capability,
			"upstream_model":    sku.UpstreamModel,
			"billing_unit":      sku.BillingUnit,
			"status":            sku.Status,
		}
		if sku.ContextWindow != nil {
			row["context_window"] = *sku.ContextWindow
		}
		if sku.MaxOutputTokens != nil {
			row["max_output_tokens"] = *sku.MaxOutputTokens
		}
		if sku.InputCents != nil {
			row["input_per_unit_cents"] = *sku.InputCents
		}
		if sku.OutputCents != nil {
			row["output_per_unit_cents"] = *sku.OutputCents
		}
		if sku.ImageCents != nil {
			row["image_per_unit_cents"] = *sku.ImageCents
		}
		// Hydrate vendor info from the static dictionary so the frontend
		// can show "火山引擎" + the console link without a second call.
		if prod, ok := catalog.LookupProduct(sku.VendorProductID); ok {
			row["vendor_product_name"] = prod.Name
			row["vendor_id"] = prod.VendorID
			if v, ok := catalog.LookupVendor(prod.VendorID); ok {
				row["vendor_name"] = v.Name
				if v.LogoURL != "" {
					row["vendor_logo_url"] = v.LogoURL
				}
			}
		}
		services = append(services, row)
	}

	cats := catalog.CategoriesSorted()
	catRows := make([]map[string]any, 0, len(cats))
	for _, c := range cats {
		catRows = append(catRows, map[string]any{
			"id":   c.ID,
			"name": c.Name,
		})
	}

	// Group capabilities by category so the modal can render a two-level
	// tree (category → 子分类).
	subRows := make([]map[string]any, 0, len(catalog.Capabilities))
	for _, c := range catalog.Capabilities {
		subRows = append(subRows, map[string]any{
			"id":           c.ID,
			"category_id":  c.CategoryID,
			"display_name": c.DisplayName,
			"billing_unit": c.BillingUnit,
		})
	}

	vendorRows := make([]map[string]any, 0, len(catalog.Vendors))
	for _, v := range catalog.Vendors {
		vendorRows = append(vendorRows, map[string]any{
			"id":   v.ID,
			"name": v.Name,
		})
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"categories":   catRows,
		"capabilities": subRows,
		"vendors":      vendorRows,
		"services":     services,
	})
}
