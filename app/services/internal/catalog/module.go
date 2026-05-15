package catalog

// ServiceModule 是「代码层注册的一个服务能力」—— 形如 `volc.ark.chat`
// 「火山·方舟·文本对话」。一个模块 = 一个 (VendorProduct, Capability) 对，
// 由对应的适配器包在编译期声明出来，admin 后台据此渲染「服务列表」并
// 提供"添加服务"入口。
//
// 关键约束：
//   - 模块的 ID = "<vendor_product_id>.<capability>"。这保证全平台唯一，
//     并且从 ID 反推 (product, capability) 不用查表。
//   - AvailableModels / AvailableRegions 是该模块向运营开放的参数空间：
//     运营在添加服务时只能从这两个清单里挑选（model 版本 + 节点）。
//     声明里没有的模型 / 节点，admin UI 上拉不出来 — 杜绝越权上架。
//   - Implemented 标记当前代码侧是否真的有可调用的适配器实现。运维
//     看到 false 的模块意味着「字典里登记了，但 internal/vendors/<id>
//     下还没写代码 / 没注册 init」—— 千万别上架，会立刻给用户报
//     no_binding_available。
//
// 演进路线：
//   未来 vendors/<vendor>/<board>/<capability>/init.go 可以在 init() 里
//   调用 RegisterAdapter(moduleID) 把自己标记为已实现，本文件的静态
//   Modules 字典就退化为「声明可见性」的清单（参数空间 + 展示）。
type ServiceModule struct {
	// ID = "<vendor_product_id>.<capability>", e.g. "volc.ark.chat".
	ID string `json:"id"`

	// VendorProductID FK 到 Products 字典。
	VendorProductID string `json:"vendor_product_id"`

	// Capability FK 到 Capabilities 字典。必须 ∈
	// Products[VendorProductID].AllowedCapabilities（init 校验）。
	Capability string `json:"capability"`

	// DisplayName 中文展示名："火山方舟 · 文本大模型"。
	DisplayName string `json:"display_name"`

	// Description 在 admin 列表里给一句话说明这是干嘛的。
	Description string `json:"description,omitempty"`

	// DefaultBillingUnit 该模块默认计费单位。如果 Capability 自己声明了
	// BillingUnit，二者应当一致 —— init 校验。
	DefaultBillingUnit string `json:"default_billing_unit"`

	// AvailableModels 是运营在「添加服务」表单里能选的模型版本列表。
	// 为空表示该模块不需要"挑模型"（如纯 ASR 实时流），admin 表单
	// 此时跳过模型选择步骤。
	AvailableModels []ModelOption `json:"available_models,omitempty"`

	// AvailableRegions 是运营能选的上游节点 / 区域。运营选哪个就把
	// 该 endpoint 落到 SKU 的 metadata，运行时凭据池据此选 binding。
	// 大多数模块至少要有 1 个（默认节点）。
	AvailableRegions []RegionOption `json:"available_regions,omitempty"`

	// SortOrder 决定 admin 列表里的展示顺序。
	SortOrder int `json:"sort_order"`
}

// ModelOption 描述一个可用的上游模型版本。
//
// ID 是平台侧的稳定 slug（落入 platform_services.id 的尾段），
// UpstreamModel 是真正传给上游 API 的 model 字段值（火山方舟带日期后缀
// 的那个）。两者分离的好处是平台 ID 可以保持简洁、可读，且日期换代
// 时换 UpstreamModel 不用改用户已开通的 entitlement 行。
type ModelOption struct {
	// ID 是平台侧的稳定 slug，e.g. "doubao-1-5-pro-32k"。
	ID string `json:"id"`
	// UpstreamModel 是上游 API 接受的 model 字段，e.g.
	// "doubao-1-5-pro-32k-250115"。
	UpstreamModel string `json:"upstream_model"`
	// DisplayName 中文展示名："豆包 1.5 Pro 32k"。
	DisplayName string `json:"display_name"`
	// ContextWindow 上下文长度（tokens）。0 = 未声明。
	ContextWindow int `json:"context_window,omitempty"`
	// MaxOutputTokens 单次响应最大输出 tokens。0 = 未声明。
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
	// DefaultInputCents / DefaultOutputCents 是模块给出的建议单价
	//（分/1k tokens）。admin 在创建 SKU 时可以一键带入；后续调价
	// 仍走 catalog.platform_pricing 历史表。0 = 不预填。
	DefaultInputCents  float64 `json:"default_input_cents,omitempty"`
	DefaultOutputCents float64 `json:"default_output_cents,omitempty"`
	// Tags 透传给 SKU 行的 tags 列，用于前端展示能力徽标。
	Tags []string `json:"tags,omitempty"`
}

// RegionOption 描述一个可用的上游节点 / 区域。Endpoint 是该区域的
// base URL，会替换 VendorProduct.EndpointTemplate 里的占位 —— 当前
// volc.ark 只有 cn-beijing 一个节点，未来加 cn-shanghai 直接在这里
// 追加一行即可。
type RegionOption struct {
	// ID 节点稳定 slug，e.g. "cn-beijing"。
	ID string `json:"id"`
	// DisplayName 中文名："华北 2（北京）"。
	DisplayName string `json:"display_name"`
	// Endpoint 该区域的上游 base URL。
	Endpoint string `json:"endpoint"`
	// Default 标记该区域是否为模块的默认推荐节点。
	Default bool `json:"default,omitempty"`
}

// Modules 是不可变的服务模块字典，键为 ServiceModule.ID。
//
// MVP：只声明 "volc.ark.chat"（火山方舟·文本大模型）。新增模块时：
//   1. 在 internal/vendors/<vendor>/<board>/<capability>/ 写出适配器
//   2. 在本文件 Modules 里追加一条声明 + 把开放的模型 / 节点写全
//   3. 如需把"代码已实现"信号自动化，让适配器 init() 调
//      RegisterAdapter(moduleID)
var Modules = map[string]ServiceModule{
	"volc.ark.chat": {
		ID:                 "volc.ark.chat",
		VendorProductID:    "volc.ark",
		Capability:         "chat",
		DisplayName:        "火山方舟 · 文本大模型",
		Description:        "豆包系列文本对话模型，OpenAI 兼容协议，支持工具调用 / JSON Mode / 流式。",
		DefaultBillingUnit: "1k_tokens",
		SortOrder:          10,
		AvailableModels: []ModelOption{
			{
				ID:                 "doubao-1-5-pro-32k",
				UpstreamModel:      "doubao-1-5-pro-32k-250115",
				DisplayName:        "豆包 1.5 Pro 32k",
				ContextWindow:      32768,
				MaxOutputTokens:    12288,
				DefaultInputCents:  0.08,
				DefaultOutputCents: 0.20,
				Tags:               []string{"tool_use", "json_mode", "streaming"},
			},
			{
				ID:                 "doubao-1-5-pro-256k",
				UpstreamModel:      "doubao-1-5-pro-256k-250115",
				DisplayName:        "豆包 1.5 Pro 256k",
				ContextWindow:      262144,
				MaxOutputTokens:    12288,
				DefaultInputCents:  0.50,
				DefaultOutputCents: 0.90,
				Tags:               []string{"tool_use", "json_mode", "streaming", "long_context"},
			},
			{
				ID:                 "doubao-1-5-lite-32k",
				UpstreamModel:      "doubao-1-5-lite-32k-250115",
				DisplayName:        "豆包 1.5 Lite 32k",
				ContextWindow:      32768,
				MaxOutputTokens:    8192,
				DefaultInputCents:  0.03,
				DefaultOutputCents: 0.06,
				Tags:               []string{"streaming"},
			},
		},
		AvailableRegions: []RegionOption{
			{
				ID:          "cn-beijing",
				DisplayName: "华北 2（北京）",
				Endpoint:    "https://ark.cn-beijing.volces.com/api/v3",
				Default:     true,
			},
		},
	},
}

// adapterRegistry 收集 init() 时各 vendors/<...> 包通过 RegisterAdapter
// 注册自己的 moduleID。值始终是 struct{}{} —— 我们只关心存在性，不存
// 附加 metadata。
var adapterRegistry = make(map[string]struct{})

// RegisterAdapter 由具体适配器包在 init() 中调用，把自己声明为"代码
// 已实现"。当前只有 volc/ark/chat 占位包，还没真接 —— 一旦它的 init
// 调本函数，admin "服务列表" 那条记录的 Implemented 就会翻成 true。
func RegisterAdapter(moduleID string) {
	adapterRegistry[moduleID] = struct{}{}
}

// IsAdapterImplemented 报告模块是否有真适配器在 init 阶段注册过。
// admin 列表用它决定那一列展示「已实现 / 未实现」。
func IsAdapterImplemented(moduleID string) bool {
	_, ok := adapterRegistry[moduleID]
	return ok
}

// LookupModule 按 ID 查模块。
func LookupModule(id string) (ServiceModule, bool) {
	m, ok := Modules[id]
	return m, ok
}

// ModulesSorted 返回按 SortOrder 排好序的模块切片，方便 admin 列表
// 直接渲染。
func ModulesSorted() []ServiceModule {
	out := make([]ServiceModule, 0, len(Modules))
	for _, m := range Modules {
		out = append(out, m)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].SortOrder > out[j].SortOrder; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
