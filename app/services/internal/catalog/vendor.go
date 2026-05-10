package catalog

// Vendor is an upstream provider (firm-level: 火山 / 阿里 / 腾讯 / OpenAI / ...).
// One vendor groups several VendorProducts (业务板块). Vendors are code
// constants because adding a new vendor always requires writing the
// master-account billing adapter and at least one product adapter.
type Vendor struct {
	// ID is the stable lower-case slug used across DB foreign keys.
	// Format: short ASCII identifier, no dots.
	ID string `json:"id"`
	// Name is the Chinese display name used in admin / market UI.
	Name string `json:"name"`
	// LogoURL is an optional CDN reference. Empty = render initials.
	LogoURL string `json:"logo_url,omitempty"`
	// ConsoleURL points to the vendor's console root, used by admin
	// UI as a quick-jump for reconciliation work.
	ConsoleURL string `json:"console_url,omitempty"`
	// MasterAuthSchema describes the credentials required to query the
	// vendor's master account (balance / billing endpoints) — distinct
	// from the per-product credentials that actually call business
	// APIs. For lightweight vendors (OpenAI / Anthropic) the schema
	// reduces to a single API key.
	MasterAuthSchema []FieldSpec `json:"master_auth_schema"`
}

// Vendors is the immutable vendor dictionary keyed by Vendor.ID.
var Vendors = map[string]Vendor{
	"volc": {
		ID:         "volc",
		Name:       "火山引擎",
		ConsoleURL: "https://console.volcengine.com/",
		// 主账号鉴权用云账号 RAM 的 ak/sk（用于查账户余额、拉账单）。
		// 业务凭据由各 VendorProduct 单独定义，不要混淆。
		MasterAuthSchema: []FieldSpec{
			{Key: "access_key_id", Label: "Access Key ID", Required: true},
			{Key: "secret_access_key", Label: "Secret Access Key", Sensitive: true, Required: true},
		},
	},
	"aliyun": {
		ID:         "aliyun",
		Name:       "阿里云",
		ConsoleURL: "https://home.console.aliyun.com/",
		MasterAuthSchema: []FieldSpec{
			{Key: "access_key_id", Label: "AccessKey ID", Required: true},
			{Key: "access_key_secret", Label: "AccessKey Secret", Sensitive: true, Required: true},
		},
	},
	"tencent": {
		ID:         "tencent",
		Name:       "腾讯云",
		ConsoleURL: "https://console.cloud.tencent.com/",
		MasterAuthSchema: []FieldSpec{
			{Key: "secret_id", Label: "SecretId", Required: true},
			{Key: "secret_key", Label: "SecretKey", Sensitive: true, Required: true},
		},
	},
	"openai": {
		ID:         "openai",
		Name:       "OpenAI",
		ConsoleURL: "https://platform.openai.com/",
		// OpenAI 没有独立的"主账号鉴权"，billing endpoint 用同一把 sk-…，
		// 这里复用为 master schema，运营侧填一次即可同时用于查余额和调业务。
		MasterAuthSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key (sk-…)", Sensitive: true, Required: true},
		},
	},
	"anthropic": {
		ID:         "anthropic",
		Name:       "Anthropic",
		ConsoleURL: "https://console.anthropic.com/",
		MasterAuthSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
	},
	"deepseek": {
		ID:         "deepseek",
		Name:       "DeepSeek",
		ConsoleURL: "https://platform.deepseek.com/",
		MasterAuthSchema: []FieldSpec{
			{Key: "api_key", Label: "API Key", Sensitive: true, Required: true},
		},
	},
}
