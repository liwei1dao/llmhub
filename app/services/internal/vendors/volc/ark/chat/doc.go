// Package chat is the placeholder package for the volc/ark chat
// capability adapter.
//
// 当前阶段 LLMHub 是「聚合 SDK」平台：SDK 拿短期 lease 直连火山方舟，平台不在
// 调用路径上。所以这里没有 adapter 代码 —— 真正的火山方舟调用在 SDK 端发生。
//
// 这个空目录的存在是为了：
//   1. 在仓库里把"火山引擎 → 方舟板块 → 大模型(chat)"这条 vendor 三层结构留出来
//   2. 等到要做的事变成需要平台代理时（例如内置 mock / 故障注入 / 巡检请求 /
//      auth_validator），代码自然落到这里，而不是和别家厂商混在一个总 adapter
//      文件里
//
// 入口 metadata 在 internal/catalog/{vendor,product,capability}.go 维护：
//   - Vendor    "volc"     (火山引擎)
//   - Product   "volc.ark" (方舟·大模型)
//   - Capability "chat"    (对话, 计费单位 1k_tokens)
package chat
