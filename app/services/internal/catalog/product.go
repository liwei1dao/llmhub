package catalog

// VendorProduct represents one "business board" (业务板块) under a
// vendor. Each product owns its own credential schema, allowed
// capability set, and protocol family — even within a single vendor,
// boards are not interchangeable (a 火山方舟 token cannot call 火山语音).
//
// Products are code constants because adding one always requires:
//   1. 在 internal/vendors/<vendor>/<board>/ 写适配器
//   2. 在本文件 Products map 里登记 metadata + credential schema
//   3. 在 internal/vendors/all.go (或同等汇总点) 注册适配器
type VendorProduct struct {
	// ID is the stable composite identifier "<vendor>.<board>", e.g.
	// "volc.ark". Used in pool.credentials.product_id and
	// catalog.platform_services.vendor_product_id.
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
	// implement. Free-form string; see internal/vendors/<id>/ for
	// accepted values.
	ProtocolFamily string `json:"protocol_family"`
	// EndpointTemplate is an optional template for the upstream base
	// URL. Concrete adapters interpolate region / cluster placeholders.
	EndpointTemplate string `json:"endpoint_template,omitempty"`
}

// Products is the immutable product dictionary keyed by VendorProduct.ID.
//
// MVP: 只有 "volc.ark"（火山方舟）。其它板块（语音 / 翻译 / 阿里 / 腾讯 / OpenAI /
// Anthropic / DeepSeek）原先列在这里只是占位，没有任何适配器代码——保留它们会
// 让运营误以为可用。等到对应 internal/vendors/<id>/<board>/ 写出适配器后再来
// 这里登记。
var Products = map[string]VendorProduct{
	"volc.ark": {
		ID:       "volc.ark",
		VendorID: "volc",
		Name:     "方舟·大模型",
		CredentialSchema: []FieldSpec{
			// 方舟 OpenAI 兼容接口的鉴权：Authorization: Bearer <api_key>。
			// 在火山控制台「方舟 → API Key 管理」里生成；老的 app_id/app_token
			// (V2/STS) 不再推荐用，SDK 直连走 OpenAI 兼容协议。
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
		AllowedCapabilities: []string{"chat"},
		ProtocolFamily:      "openai_compat",
		EndpointTemplate:    "https://ark.cn-beijing.volces.com/api/v3",
	},
}
