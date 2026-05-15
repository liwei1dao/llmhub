package admin

import (
	"context"
	"net/http"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// ServiceModuleView 是 admin 「服务列表」页一行的载荷：模块声明 +
// 运行时计算字段（adapter 是否注册、当前已上架几条 SKU）。
//
// listed_skus 是这条模块当前在 catalog.platform_services 表里已经
// 落了几行（用 vendor_product_id + capability 联合过滤）。这是运
// 营最关心的"这个模块上架到什么程度了"的信号。
type ServiceModuleView struct {
	ID                 string                  `json:"id"`
	VendorProductID    string                  `json:"vendor_product_id"`
	VendorProductName  string                  `json:"vendor_product_name,omitempty"`
	VendorID           string                  `json:"vendor_id,omitempty"`
	VendorName         string                  `json:"vendor_name,omitempty"`
	Capability         string                  `json:"capability"`
	CapabilityName     string                  `json:"capability_name,omitempty"`
	CategoryID         string                  `json:"category_id,omitempty"`
	CategoryName       string                  `json:"category_name,omitempty"`
	DisplayName        string                  `json:"display_name"`
	Description        string                  `json:"description,omitempty"`
	DefaultBillingUnit string                  `json:"default_billing_unit"`
	SortOrder          int                     `json:"sort_order"`
	Implemented        bool                    `json:"implemented"`
	AvailableModels    []catalog.ModelOption   `json:"available_models,omitempty"`
	AvailableRegions   []catalog.RegionOption  `json:"available_regions,omitempty"`
	ListedSKUs         int                     `json:"listed_skus"`
}

// listServiceModules 返回 admin 「服务列表」页所需的全部数据：
//   - 每个代码侧注册的 ServiceModule 一行
//   - 附上对应 product / vendor / capability / category 的展示名
//   - implemented 字段告诉运营适配器是否真的在 init() 注册过
//   - listed_skus 字段返回该模块在 catalog.platform_services 已经上架的 SKU 数
//
// 没有筛选 / 分页 —— 模块清单是代码常量，量级永远很小（个位数到几十）。
func (s *Server) listServiceModules(w http.ResponseWriter, r *http.Request) {
	mods := catalog.ModulesSorted()

	// 一次性统计每个 (vendor_product, capability) 在 platform_services 表里
	// 的 SKU 数，避免 N+1。表本身行数不大（运营级），全表 GROUP BY 足够。
	counts := make(map[string]int, len(mods))
	if s.pool != nil {
		const sql = `
SELECT vendor_product_id || '.' || capability AS module_id, COUNT(*)
FROM catalog.platform_services
GROUP BY vendor_product_id, capability
`
		pp := s.pool.Repo().Pool()
		rows, err := pp.Query(r.Context(), sql)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		for rows.Next() {
			var k string
			var c int
			if err := rows.Scan(&k, &c); err != nil {
				rows.Close()
				httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
				return
			}
			counts[k] = c
		}
		rows.Close()
	}

	out := make([]ServiceModuleView, 0, len(mods))
	for _, m := range mods {
		v := ServiceModuleView{
			ID:                 m.ID,
			VendorProductID:    m.VendorProductID,
			Capability:         m.Capability,
			DisplayName:        m.DisplayName,
			Description:        m.Description,
			DefaultBillingUnit: m.DefaultBillingUnit,
			SortOrder:          m.SortOrder,
			Implemented:        catalog.IsAdapterImplemented(m.ID),
			AvailableModels:    m.AvailableModels,
			AvailableRegions:   m.AvailableRegions,
			ListedSKUs:         counts[m.ID],
		}
		if p, ok := catalog.LookupProduct(m.VendorProductID); ok {
			v.VendorProductName = p.Name
			v.VendorID = p.VendorID
			if vd, ok := catalog.LookupVendor(p.VendorID); ok {
				v.VendorName = vd.Name
			}
		}
		if c, ok := catalog.LookupCapability(m.Capability); ok {
			v.CapabilityName = c.DisplayName
			v.CategoryID = c.CategoryID
			if cat, ok := catalog.LookupCategory(c.CategoryID); ok {
				v.CategoryName = cat.Name
			}
		}
		out = append(out, v)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

var _ context.Context // keep import if future signature changes need it
