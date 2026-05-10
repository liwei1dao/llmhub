package catalog

// Capability is one upstream business capability (e.g. chat / embedding /
// asr_realtime / tts_voice_clone). Capabilities are code constants because
// each one needs an adapter implementation per VendorProduct.
//
// A Capability belongs to exactly one PlatformCategory. The
// (vendor_product, capability) pair is the routing key — one row in
// pool.credential_services per pair is the smallest schedulable unit.
type Capability struct {
	// ID is the stable lower-case slug, e.g. "chat" / "asr_realtime".
	// Used in pool.credential_services.capability and as a routing key.
	ID string `json:"id"`
	// CategoryID points to a PlatformCategory.ID.
	CategoryID string `json:"category_id"`
	// DisplayName is the short Chinese description for admin UI.
	DisplayName string `json:"display_name"`
	// BillingUnit is the natural unit upstream charges in.
	// Conventional values: "1k_tokens" / "minute" / "1k_chars" /
	// "image" / "page" / "query".
	BillingUnit string `json:"billing_unit"`
}

// Capabilities is the immutable capability dictionary keyed by Capability.ID.
//
// MVP: 只激活 "chat"（大模型对话）。embedding / vision / image_gen / asr_* /
// tts_* / mt_* 这些原先列在这里的能力全都没有 adapter，先全部摘掉避免误用。
// 新增能力时同步在 internal/vendors/<id>/<board>/<capability>/ 写适配器。
var Capabilities = map[string]Capability{
	"chat": {ID: "chat", CategoryID: "llm", DisplayName: "对话", BillingUnit: "1k_tokens"},
}
