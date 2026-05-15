package chat

import (
	"errors"
	"strings"

	"github.com/llmhub/llmhub/internal/catalog"
)

// ModuleID 是这个适配器对应的服务模块 ID，和 catalog.Modules
// 里登记的键一致。集中放一个常量，避免 init 注册 / 校验代码 / 日志
// 里到处出现字面量。
const ModuleID = "volc.ark.chat"

// init 在程序启动时把本适配器登记到 catalog.adapterRegistry。
// admin「服务列表」据此把这一行的「未实现 / 代码已实现」徽标翻成
// 绿色，"添加服务"按钮也据此解禁。
//
// 注意：catalog 包 init() 已经先跑过（go 的 import 关系决定了依赖
// 先初始化），所以此处可以安全调 RegisterAdapter。
func init() {
	catalog.RegisterAdapter(ModuleID)
}

// ErrMissingAPIKey 是 ValidateAuthPayload 在凭据缺 api_key 字段（或
// 字段为空白字符）时返回的错误。
var ErrMissingAPIKey = errors.New("volc.ark: api_key is required")

// ValidateAuthPayload 在录入凭据 / 颁发 lease 前做一次本地形态校验，
// 防止"程序绕过 admin 表单 schema 直接写库"造成的脏数据。本函数不
// 联网验证 api_key 是否真有效 —— 那是 vendor/<id>/auth_validator/
// 未来的职责，需要一次小代价的探针请求才有意义。
//
// 入参是从 vault 解出的 auth_payload map。
func ValidateAuthPayload(payload map[string]string) error {
	if payload == nil {
		return ErrMissingAPIKey
	}
	key, ok := payload["api_key"]
	if !ok || strings.TrimSpace(key) == "" {
		return ErrMissingAPIKey
	}
	return nil
}
