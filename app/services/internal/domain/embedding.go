package domain

// EmbeddingRequest is the platform-internal embedding request, shaped
// like OpenAI's /v1/embeddings.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"` // "float" (default) | "base64"
	Dimensions     int      `json:"dimensions,omitempty"`
	User           string   `json:"user,omitempty"`
}

// EmbeddingResponse mirrors the OpenAI /v1/embeddings output.
type EmbeddingResponse struct {
	Object string         `json:"object"` // always "list"
	Data   []EmbeddingRow `json:"data"`
	Model  string         `json:"model"`
	Usage  Usage          `json:"usage"`
}

// EmbeddingRow is one vector entry in the response.
type EmbeddingRow struct {
	Object    string    `json:"object"` // always "embedding"
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}
