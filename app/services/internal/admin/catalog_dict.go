package admin

import (
	"net/http"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// listCategories returns the platform-category dictionary in display
// order. Drives the marketplace navigation tree and the admin
// "服务分类" listing.
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"data": catalog.CategoriesSorted()})
}

// listVendors returns every vendor along with its master-auth schema
// and grouped products. Each vendor entry embeds its products so the
// admin UI can render the厂商目录 cards in one round-trip.
func (s *Server) listVendors(w http.ResponseWriter, r *http.Request) {
	groups := catalog.ProductsByVendor()
	type vendorView struct {
		catalog.Vendor
		Products []catalog.VendorProduct `json:"products"`
	}
	out := make([]vendorView, 0, len(catalog.Vendors))
	for id, v := range catalog.Vendors {
		out = append(out, vendorView{Vendor: v, Products: groups[id]})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// listProducts returns the flat business-board (业务板块) dictionary.
// Useful for the standalone "板块" admin page and for the credential
// wizard's step 2.
func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	out := make([]catalog.VendorProduct, 0, len(catalog.Products))
	for _, p := range catalog.Products {
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// listCapabilities returns the upstream-capability dictionary, used
// by the admin "上游能力" page and as a lookup for binding creation.
func (s *Server) listCapabilities(w http.ResponseWriter, r *http.Request) {
	out := make([]catalog.Capability, 0, len(catalog.Capabilities))
	for _, c := range catalog.Capabilities {
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}
