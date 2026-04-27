// Package deepl is the adapter for DeepL text translation.
// TODO(V1): wire translate capability under deepl/translate/.
package deepl

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/stub"
)

const ID = "deepl"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return stub.New(cfg), nil
	})
}
