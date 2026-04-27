// Package dashscope is the adapter for Aliyun Bailian / DashScope.
// TODO(M6): wire real chat / asr / tts adapters.
package dashscope

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/stub"
)

const ID = "dashscope"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return stub.New(cfg), nil
	})
}
