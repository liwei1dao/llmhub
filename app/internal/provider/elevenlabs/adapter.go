// Package elevenlabs is the adapter for ElevenLabs text-to-speech.
// TODO(V1): wire tts capability under elevenlabs/tts/.
package elevenlabs

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/stub"
)

const ID = "elevenlabs"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return stub.New(cfg), nil
	})
}
