package catalog

// Capability is one upstream business capability (e.g. chat /
// asr_realtime / tts_voice_clone). Capabilities are code constants
// because each one needs an adapter implementation per VendorProduct.
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
var Capabilities = map[string]Capability{
	// ── 大模型 ─────────────────────────────────────────
	"chat":      {ID: "chat", CategoryID: "llm", DisplayName: "对话", BillingUnit: "1k_tokens"},
	"embedding": {ID: "embedding", CategoryID: "llm", DisplayName: "向量化", BillingUnit: "1k_tokens"},
	"rerank":    {ID: "rerank", CategoryID: "llm", DisplayName: "重排", BillingUnit: "query"},
	"vision":    {ID: "vision", CategoryID: "llm", DisplayName: "多模态", BillingUnit: "1k_tokens"},
	"image_gen": {ID: "image_gen", CategoryID: "llm", DisplayName: "文生图", BillingUnit: "image"},
	// ── 语音识别 ─────────────────────────────────────
	"asr_realtime": {ID: "asr_realtime", CategoryID: "asr", DisplayName: "实时识别", BillingUnit: "minute"},
	"asr_offline":  {ID: "asr_offline", CategoryID: "asr", DisplayName: "录音文件", BillingUnit: "minute"},
	"asr_whisper":  {ID: "asr_whisper", CategoryID: "asr", DisplayName: "Whisper", BillingUnit: "minute"},
	// ── 语音合成 ─────────────────────────────────────
	"tts_standard":    {ID: "tts_standard", CategoryID: "tts", DisplayName: "标准音色", BillingUnit: "1k_chars"},
	"tts_voice_clone": {ID: "tts_voice_clone", CategoryID: "tts", DisplayName: "声音克隆", BillingUnit: "1k_chars"},
	"tts_longform":    {ID: "tts_longform", CategoryID: "tts", DisplayName: "长文本合成", BillingUnit: "1k_chars"},
	// ── 翻译 ────────────────────────────────────────
	"mt_text":     {ID: "mt_text", CategoryID: "mt", DisplayName: "文本翻译", BillingUnit: "1k_chars"},
	"mt_document": {ID: "mt_document", CategoryID: "mt", DisplayName: "文档翻译", BillingUnit: "page"},
}
