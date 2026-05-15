// Package vendors 是「所有上游适配器」的聚合入口。
//
// 各家厂商的适配器代码住在 vendors/<vendor>/<board>/<capability>/，
// 它们在 init() 里通过 catalog.RegisterAdapter("<module_id>") 把自己
// 声明为"代码已实现"。要让那个 init 真的被触发，本文件用 blank import
// 把所有适配器子包拉进来 —— main.go 只要 import 一次本聚合包，整条
// adapter 注册链就会跑齐。
//
// 加新适配器的时候只要在下面 import 块里追加一行；不需要改 main.go。
//
//   import _ "github.com/llmhub/llmhub/internal/vendors/<new_path>"
package vendors

import (
	// 火山引擎 · 方舟 · 文本对话
	_ "github.com/llmhub/llmhub/internal/vendors/volc/ark/chat"
)
