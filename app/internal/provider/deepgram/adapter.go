// Package deepgram is the adapter for Deepgram speech recognition.
// TODO(V1): wire asr capability under deepgram/asr/.
package deepgram

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/stub"
)

const ID = "deepgram"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return stub.New(cfg), nil
	})
}
