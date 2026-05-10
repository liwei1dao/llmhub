package catalog

// PlatformCategory is a top-level platform classification. Categories
// are what end users see in the model marketplace ("大模型"/"语音识别"/
// "语音合成"/"翻译"); they are independent from upstream vendor
// taxonomy. Adding a category requires changes to the marketplace UI
// and is therefore a code constant — not a DB row.
type PlatformCategory struct {
	ID        string `json:"id"`         // Stable id used in DB foreign keys (catalog.platform_services.category_id).
	Name      string `json:"name"`       // Chinese display name.
	SortOrder int    `json:"sort_order"` // Lower = earlier in marketplace navigation.
}

// Categories is the immutable platform-category dictionary. Order in
// the source map is irrelevant; SortOrder governs display.
var Categories = map[string]PlatformCategory{
	"llm": {ID: "llm", Name: "大模型", SortOrder: 10},
	"asr": {ID: "asr", Name: "语音识别", SortOrder: 20},
	"tts": {ID: "tts", Name: "语音合成", SortOrder: 30},
	"mt":  {ID: "mt", Name: "翻译", SortOrder: 40},
}

// CategoriesSorted returns the category dictionary as a slice ordered
// by SortOrder, suitable for direct use in admin / marketplace
// listings without re-sorting at every call site.
func CategoriesSorted() []PlatformCategory {
	out := make([]PlatformCategory, 0, len(Categories))
	for _, c := range Categories {
		out = append(out, c)
	}
	sortByCategoryOrder(out)
	return out
}

func sortByCategoryOrder(s []PlatformCategory) {
	// Tiny n (4) — insertion sort keeps the helper allocation-free
	// and avoids pulling in sort just for this.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].SortOrder > s[j].SortOrder; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
