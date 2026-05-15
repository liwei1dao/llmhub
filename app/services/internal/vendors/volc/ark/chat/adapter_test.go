package chat

import (
	"testing"

	"github.com/llmhub/llmhub/internal/catalog"
)

// TestAdapterRegistered 确认本包的 init() 真的把自己挂上了
// catalog.adapterRegistry。这条断言看起来很微小，但它是 admin
// 「服务列表」上"代码已实现"徽标和"添加服务"按钮解锁的唯一信号 ——
// 如果哪天有人把 init() 拆走又忘了挂回去，这个测试会立刻报警。
func TestAdapterRegistered(t *testing.T) {
	if !catalog.IsAdapterImplemented(ModuleID) {
		t.Fatalf("expected adapter %q to be registered via init()", ModuleID)
	}
}

func TestValidateAuthPayload(t *testing.T) {
	cases := []struct {
		name    string
		in      map[string]string
		wantErr bool
	}{
		{"nil", nil, true},
		{"missing api_key", map[string]string{"other": "x"}, true},
		{"blank api_key", map[string]string{"api_key": "   "}, true},
		{"ok", map[string]string{"api_key": "sk-volc-xxxx"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateAuthPayload(c.in)
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
