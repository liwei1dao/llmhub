package catalog

// Vendor is an upstream provider (firm-level entity: 火山 / 阿里 / 腾讯 / OpenAI / ...).
// One vendor groups several VendorProducts (业务板块). Vendors are code
// constants because adding a new vendor always requires:
//
//  1. 在 internal/vendors/<id>/ 写新的厂商目录（账号鉴权 schema、业务板块、能力适配器）
//  2. 在本文件 Vendors map 里登记一条 metadata
//  3. 如果有自动余额/账单读取，挂上 billing adapter
//
// 当前 MVP 阶段只激活 volc 一家。其它厂商在 internal/vendors/ 下还没写适配器，
// 因此不放进字典；运营在新增账号表单里看不到它们。
//
// 之前曾经登记过的 aliyun / tencent / openai / anthropic / deepseek metadata
// 一并移除——保留它们没有意义，反而误导运营以为可用。
type Vendor struct {
	// ID is the stable lower-case slug used across DB foreign keys.
	// Format: short ASCII identifier, no dots.
	ID string `json:"id"`
	// Name is the Chinese display name used in admin / market UI.
	Name string `json:"name"`
	// LogoURL is an optional CDN reference. Empty = render initials.
	LogoURL string `json:"logo_url,omitempty"`
	// ConsoleURL points to the vendor's official site / console, used by admin
	// UI as a quick-jump for reconciliation work.
	ConsoleURL string `json:"console_url,omitempty"`
	// MasterAuthSchema describes the credentials required to query the
	// vendor's master account (balance / billing endpoints) — distinct
	// from the per-product credentials that actually call business
	// APIs. Each vendor designs its own schema; do not assume a shared
	// shape across vendors.
	MasterAuthSchema []FieldSpec `json:"master_auth_schema"`
}

// Vendors is the immutable vendor dictionary keyed by Vendor.ID.
//
// MVP: 只有 "volc"（火山引擎）。新增厂商需要在 internal/vendors/<id>/ 落地
// 适配器之后再来这里登记。
var Vendors = map[string]Vendor{
	"volc": {
		ID:         "volc",
		Name:       "火山引擎",
		ConsoleURL: "https://www.volcengine.com/",
		// 当前阶段：仅记录控制台「登录账号 + 密码」，供运营手工巡检 / 后台对账时使用。
		// 自动抓余额 / 拉账单的能力尚未接入；接入后会以追加字段（如 AK/SK 或 OAuth
		// token）扩展本 schema，旧记录的 auth_payload 不强制迁移，billing 适配器按需
		// 检查可用字段。每家厂商的鉴权字段独立设计，不要塞进通用列表。
		MasterAuthSchema: []FieldSpec{
			{Key: "account", Label: "登录账号（手机号/邮箱）", Required: true},
			{Key: "password", Label: "登录密码", Sensitive: true, Required: true},
		},
	},
}
