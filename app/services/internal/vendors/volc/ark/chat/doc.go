// Package chat 是火山引擎 · 方舟 · 文本对话能力的服务端适配器。
//
// LLMHub 是「聚合 SDK」平台：SDK 拿短期 lease 直连火山方舟，平台不在
// 调用路径上。所以这个包不写"调上游 API"的代码 —— 那一段在
// sdk/go/transport_openai_compat.go。
//
// 这个包真正承担的职责：
//
//  1. init() 时调 catalog.RegisterAdapter("volc.ark.chat")，向平台
//     声明"代码侧已实现这个服务模块" —— admin「服务列表」据此把
//     Implemented 标记翻成 true，"添加服务"按钮才会解锁。
//
//  2. ValidateAuthPayload 在 admin 录入凭据 / SDK 颁发 lease 之前
//     做一次本地形态校验：保证 api_key 字段存在且非空。这层校验是
//     防御性的 —— catalog.Products["volc.ark"].CredentialSchema 已经
//     声明了 api_key 是 required，但 schema 检查依赖 admin 表单层；
//     在适配器里再校一次能挡住"程序直接绕过表单写库"的情形。
//
// 入口 metadata 在 internal/catalog/{vendor,product,capability,module}.go：
//   - Vendor      "volc"          (火山引擎)
//   - Product     "volc.ark"      (方舟·大模型)
//   - Capability  "chat"          (对话, 计费单位 1k_tokens)
//   - Module      "volc.ark.chat" (火山方舟·文本大模型) — 由本包注册
package chat
