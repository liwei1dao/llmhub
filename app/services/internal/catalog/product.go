package catalog

// VendorProduct represents one "business board" (业务板块) under a
// vendor. Each product owns its own credential schema, allowed
// capability set, and protocol family — even within a single vendor,
// boards are not interchangeable (a 火山方舟 token cannot call 火山语音).
//
// Products are code constants because adding one always requires:
//   1. writing the credential schema here,
//   2. writing the capability adapters in internal/upstream/<vendor>/<board>/,
//   3. registering them with the upstream registry at startup.
type VendorProduct struct {
	// ID is the stable composite identifier "<vendor>.<board>", e.g.
	// "volc.ark", "aliyun.nls". Used in pool.credentials.product_id
	// and catalog.platform_services.vendor_product_id.
	ID string `json:"id"`
	// VendorID must match a Vendor.ID.
	VendorID string `json:"vendor_id"`
	// Name is the Chinese display name shown in admin UI.
	Name string `json:"name"`
	// CredentialSchema declares the per-product credential fields a
	// human operator must fill when registering a credential under
	// this product. Order is preserved in the admin form.
	CredentialSchema []FieldSpec `json:"credential_schema"`
	// AllowedCapabilities lists the Capability.IDs that can be bound
	// to a credential of this product. The runtime invariant
	//   pool.credential_services.capability ∈ AllowedCapabilities
	// is enforced by the application layer at binding-create time.
	AllowedCapabilities []string `json:"allowed_capabilities"`
	// ProtocolFamily picks the wire protocol family the adapter must
	// implement. Free-form string; see internal/upstream registry
	// for accepted values.
	ProtocolFamily string `json:"protocol_family"`
	// EndpointTemplate is an optional template for the upstream base
	// URL. Concrete adapters interpolate region / cluster placeholders.
	EndpointTemplate string `json:"endpoint_template,omitempty"`
}

// Products is the immutable product dictionary keyed by VendorProduct.ID.
//
// Twelve products as of v0.2:
//   3 × volc      (ark / speech / translate)
//   3 × aliyun    (dashscope / nls / mt)
//   3 × tencent   (hunyuan / speech / mt)
//   1 × openai    (api)
//   1 × anthropic (api)
//   1 × deepseek  (api)
var Products = map[string]VendorProduct{
	// ── volc ────────────────────────────────────────────────────
	"volc.ark": {
		ID:       "volc.ark",
		VendorID: "volc",
		Name:     "方舟·大模型",
		CredentialSchema: []FieldSpec{
			{Key: "app_id", Label: "App ID", Required: true},
			{Key: "app_token", Label: "App Token", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "embedding", "vision", "image_gen"},
		ProtocolFamily:      "openai_compat",
		EndpointTemplate:    "https://ark.cn-beijing.volces.com/api/v3",
	},
	"volc.speech": {
		ID:       "volc.speech",
		VendorID: "volc",
		Name:     "语音技术",
		CredentialSchema: []FieldSpec{
			{Key: "appid", Label: "AppID", Required: true},
			{Key: "access_token", Label: "Access Token", Sensitive: true, Required: true},
			{Key: "cluster", Label: "Cluster", Required: true},
		},
		AllowedCapabilities: []string{"asr_realtime", "asr_offline", "tts_standard", "tts_voice_clone"},
		ProtocolFamily:      "volc_signed_v4",
		EndpointTemplate:    "wss://openspeech.bytedance.com",
	},
	"volc.translate": {
		ID:       "volc.translate",
		VendorID: "volc",
		Name:     "机器翻译",
		CredentialSchema: []FieldSpec{
			{Key: "access_key_id", Label: "Access Key ID", Required: true},
			{Key: "secret_access_key", Label: "Secret Access Key", Sensitive: true, Required: true},
			{Key: "region", Label: "区域", Required: true, Pattern: "^[a-z]+-[a-z0-9-]+$"},
		},
		AllowedCapabilities: []string{"mt_text", "mt_document"},
		ProtocolFamily:      "volc_signed_v4",
		EndpointTemplate:    "https://translate.volcengineapi.com",
	},

	// ── aliyun ──────────────────────────────────────────────────
	"aliyun.dashscope": {
		ID:       "aliyun.dashscope",
		VendorID: "aliyun",
		Name:     "百炼·大模型",
		CredentialSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "embedding", "rerank"},
		ProtocolFamily:      "openai_compat",
		EndpointTemplate:    "https://dashscope.aliyuncs.com/compatible-mode/v1",
	},
	"aliyun.nls": {
		ID:       "aliyun.nls",
		VendorID: "aliyun",
		Name:     "智能语音 NLS",
		CredentialSchema: []FieldSpec{
			{Key: "app_key", Label: "AppKey", Required: true},
			// access_token 由 STS 临时签发，需要定期刷新；schema 里仍按
			// 静态 token 录入，刷新策略由 adapter 内部处理。
			{Key: "access_token", Label: "Access Token", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"asr_realtime", "asr_offline", "tts_standard"},
		ProtocolFamily:      "aliyun_nls_ws",
		EndpointTemplate:    "wss://nls-gateway.cn-shanghai.aliyuncs.com",
	},
	"aliyun.mt": {
		ID:       "aliyun.mt",
		VendorID: "aliyun",
		Name:     "机器翻译",
		CredentialSchema: []FieldSpec{
			{Key: "access_key_id", Label: "AccessKey ID", Required: true},
			{Key: "access_key_secret", Label: "AccessKey Secret", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"mt_text", "mt_document"},
		ProtocolFamily:      "aliyun_pop",
		EndpointTemplate:    "https://mt.aliyuncs.com",
	},

	// ── tencent ─────────────────────────────────────────────────
	"tencent.hunyuan": {
		ID:       "tencent.hunyuan",
		VendorID: "tencent",
		Name:     "混元·大模型",
		CredentialSchema: []FieldSpec{
			{Key: "secret_id", Label: "SecretId", Required: true},
			{Key: "secret_key", Label: "SecretKey", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "vision"},
		ProtocolFamily:      "tencent_signed_v3",
		EndpointTemplate:    "https://hunyuan.tencentcloudapi.com",
	},
	"tencent.speech": {
		ID:       "tencent.speech",
		VendorID: "tencent",
		Name:     "语音 ASR/TTS",
		CredentialSchema: []FieldSpec{
			{Key: "app_id", Label: "AppId", Required: true},
			{Key: "secret_id", Label: "SecretId", Required: true},
			{Key: "secret_key", Label: "SecretKey", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"asr_realtime", "asr_offline", "tts_standard"},
		ProtocolFamily:      "tencent_signed_v3",
		EndpointTemplate:    "wss://asr.cloud.tencent.com",
	},
	"tencent.mt": {
		ID:       "tencent.mt",
		VendorID: "tencent",
		Name:     "机器翻译",
		CredentialSchema: []FieldSpec{
			{Key: "secret_id", Label: "SecretId", Required: true},
			{Key: "secret_key", Label: "SecretKey", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"mt_text", "mt_document"},
		ProtocolFamily:      "tencent_signed_v3",
		EndpointTemplate:    "https://tmt.tencentcloudapi.com",
	},

	// ── openai / anthropic / deepseek (单板块) ──────────────────
	"openai.api": {
		ID:       "openai.api",
		VendorID: "openai",
		Name:     "OpenAI API",
		CredentialSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key (sk-…)", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "embedding", "image_gen", "asr_whisper", "tts_standard"},
		ProtocolFamily:      "openai_native",
		EndpointTemplate:    "https://api.openai.com/v1",
	},
	"anthropic.api": {
		ID:       "anthropic.api",
		VendorID: "anthropic",
		Name:     "Anthropic API",
		CredentialSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "vision"},
		ProtocolFamily:      "anthropic_native",
		EndpointTemplate:    "https://api.anthropic.com/v1",
	},
	"deepseek.api": {
		ID:       "deepseek.api",
		VendorID: "deepseek",
		Name:     "DeepSeek API",
		CredentialSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat", "embedding"},
		ProtocolFamily:      "openai_compat",
		EndpointTemplate:    "https://api.deepseek.com/v1",
	},
}

// ProductsByVendor groups products under their owning vendor in a
// stable order (the order they appear in Products is preserved within
// each vendor, vendors themselves come out in Products' iteration
// order — callers that need deterministic ordering should sort the
// outer keys).
func ProductsByVendor() map[string][]VendorProduct {
	out := make(map[string][]VendorProduct, len(Vendors))
	for _, p := range Products {
		out[p.VendorID] = append(out[p.VendorID], p)
	}
	return out
}
