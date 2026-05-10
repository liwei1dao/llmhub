package account

import (
	"net/http"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// SDKReleaseInfo describes one downloadable SDK build. The API base
// URL is included so the SDK can be pre-configured to talk to the
// right environment (local / staging / prod) without a separate
// `LLMHUB_API_BASE` env var that the user has to set.
type SDKReleaseInfo struct {
	Language     string `json:"language"`      // go / typescript / python / java
	DisplayName  string `json:"display_name"`  // "LLMHub Go SDK"
	Version      string `json:"version"`       // semver
	ReleasedAt   string `json:"released_at"`   // RFC3339
	DownloadURL  string `json:"download_url"`  // signed URL or static asset
	Checksum     string `json:"checksum"`      // sha256:abc...
	APIBaseURL   string `json:"api_base_url"`  // baked-in default the SDK should hit
	InstallHint  string `json:"install_hint"`  // "go get ...", "npm i ...", "pip install ..."
	DocsURL      string `json:"docs_url,omitempty"`
	Status       string `json:"status"`        // active / beta / planned
}

// handleSDKDownloads returns the SDK distribution manifest for the
// authenticated user. v1 of this endpoint is a static manifest; in M-future
// it'll be backed by a `sdk_releases` table maintained by ops, so we can
// stage / canary releases per-user.
func (s *Server) handleSDKDownloads(w http.ResponseWriter, r *http.Request) {
	apiBase := defaultAPIBaseFromRequest(r)
	out := []SDKReleaseInfo{
		{
			Language:    "go",
			DisplayName: "LLMHub Go SDK",
			Version:     "0.1.0",
			ReleasedAt:  "2026-05-09T00:00:00Z",
			Status:      "beta",
			APIBaseURL:  apiBase,
			InstallHint: "go get github.com/llmhub/llmhub-go-sdk@v0.1.0",
			DownloadURL: "https://github.com/llmhub/llmhub-go-sdk/releases/tag/v0.1.0",
			DocsURL:     "/docs/sdk/go",
		},
		{
			Language:    "typescript",
			DisplayName: "LLMHub Node SDK",
			Version:     "0.0.0",
			Status:      "planned",
			APIBaseURL:  apiBase,
			InstallHint: "npm i @llmhub/sdk",
		},
		{
			Language:    "python",
			DisplayName: "LLMHub Python SDK",
			Version:     "0.0.0",
			Status:      "planned",
			APIBaseURL:  apiBase,
			InstallHint: "pip install llmhub-sdk",
		},
		{
			Language:    "java",
			DisplayName: "LLMHub Java SDK",
			Version:     "0.0.0",
			Status:      "planned",
			APIBaseURL:  apiBase,
			InstallHint: "implementation 'com.llmhub:sdk:0.0.0'",
		},
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// defaultAPIBaseFromRequest reflects the request's scheme + host back so
// the SDK is auto-configured for whichever environment the user
// downloaded it from. Falls back to the request Host if no proxy
// X-Forwarded-Proto header is present.
func defaultAPIBaseFromRequest(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}
