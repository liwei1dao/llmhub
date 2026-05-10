package domain

// ProviderMeta describes a registered upstream provider.
type ProviderMeta struct {
	ID                    string
	DisplayName           string
	BaseURL               string
	AuthMode              string // "bearer" / "ak_sk" / "signed_token"
	ProtocolFamily        string // "openai_compat" / "anthropic" / "custom"
	Status                string // "active" / "paused" / "deprecated"
	SupportedCapabilities []string
}

// LogicalModel is the platform-facing model a user sees.
// It can map to multiple upstream models across providers.
type LogicalModel struct {
	ID            string
	DisplayName   string
	CapabilityID  string
	Category      string
	Capabilities  []string
	ContextWindow int
	MaxOutput     int
	IsPublic      bool
}

// ModelMapping ties a logical model to an upstream model on a provider.
type ModelMapping struct {
	ModelID       string
	ProviderID    string
	UpstreamModel string
	Priority      int
	Status        string
}
