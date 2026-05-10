package domain

import "io"

// ASRRequest is the unified speech recognition request.
type ASRRequest struct {
	Model        string
	Language     string // "zh", "en", "auto"
	Format       string // "json" / "verbose_json" / "srt" / "vtt"
	Granularity  []string
	Audio        io.Reader // batch mode
	SampleRate   int
	Channels     int
}

// ASRResponse is the unified speech recognition response.
type ASRResponse struct {
	Text     string
	Segments []ASRSegment
	Duration float64
}

// ASRSegment is a time-bounded chunk of recognized text.
type ASRSegment struct {
	Text  string
	Start float64
	End   float64
}

// ASRStreamRequest describes a streaming ASR session.
type ASRStreamRequest struct {
	Model      string
	Language   string
	SampleRate int
	Channels   int
}

// UpstreamStreamEndpoint is returned by a provider's streaming adapter.
// The gateway uses this to proxy the client's WebSocket to the upstream.
type UpstreamStreamEndpoint struct {
	URL     string            // wss://... with short-lived token
	Headers map[string]string // extra headers to set on dial
	TokenTTLSeconds int
}

// TTSRequest is the unified speech synthesis request.
type TTSRequest struct {
	Model   string
	Input   string
	VoiceID string // logical voice id; adapter maps to upstream voice
	Format  string // "mp3" / "wav" / "opus" / "pcm"
	Speed   float32
	Emotion string
	Stream  bool
}

// TTSResponse for non-streaming mode.
type TTSResponse struct {
	Audio       []byte
	ContentType string
	Characters  int
}
