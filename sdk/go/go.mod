// Module path mirrors the eventual public path so users can `go get
// github.com/llmhub/llmhub-go-sdk` once we cut a standalone repo.
// Until then the SDK lives in the monorepo as its own module so its
// dependency graph stays clean (the SDK must NEVER import anything
// from internal/* of the platform backend).
module github.com/llmhub/llmhub-go-sdk

go 1.22
