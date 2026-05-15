# LLMHub Go SDK

The Go client SDK for the [LLMHub aggregation platform](https://llmhub.com).

```
                ┌──────────────────────────────┐
   your code ─→ │  llmhub.Client (this SDK)    │ ─→ upstream (火山 / DeepSeek / …)
                │   ─ leases real upstream key │     direct call, no proxy
                │   ─ async usage report       │
                └──────────────────────────────┘
                              │
                              ▼
                    POST /sdk/credentials/issue
                    POST /sdk/usage/report
                    https://api.llmhub.com
```

## How it works

1. You sign up on the platform console and get a single `api_key`.
2. You subscribe to the SKUs you want (e.g. `deepseek-chat`, `doubao-pro`).
3. Your code calls `client.Chat(model: "deepseek-chat", …)`.
4. The SDK trades `(api_key, sku_id)` for a 15-minute **lease** containing
   the *real* upstream credential.
5. The SDK calls the upstream provider directly and returns the response
   to you.
6. The SDK asynchronously reports the call back to the platform so your
   subscription quota gets decremented.

The platform never sees your prompts or responses. Subscription quota
is decremented from `/sdk/usage/report` events only.

## Install

```sh
go get github.com/llmhub/llmhub-go-sdk@latest
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "os"

    llmhub "github.com/llmhub/llmhub-go-sdk"
)

func main() {
    c, err := llmhub.New(llmhub.Config{
        APIKey: os.Getenv("LLMHUB_API_KEY"),
    })
    if err != nil { panic(err) }
    defer c.Close()

    resp, err := c.Chat(context.Background(), &llmhub.ChatRequest{
        Model: "deepseek-chat",
        Messages: []llmhub.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })
    if err != nil { panic(err) }
    fmt.Println(resp.Choices[0].Message.Content)
}
```

Streaming:

```go
s, _ := c.ChatStream(ctx, &llmhub.ChatRequest{Model: "deepseek-chat", Messages: msgs})
for chunk := range s.Chunks() {
    if chunk.Delta != nil {
        fmt.Print(chunk.Delta.Content)
    }
}
if err := s.Err(); err != nil { /* ... */ }
```

## Supported SKUs

The SDK ships with a single `openai_compat` transport that covers most
LLM providers in the platform's catalog:

| Provider | SKUs (examples) |
|----------|-----------------|
| DeepSeek | `deepseek-chat` |
| 火山方舟 | `doubao-pro`, `doubao-lite-32k`, … |
| 阿里 DashScope | `qwen-plus`, `qwen-max`, … |

ASR / TTS / translation SKUs require additional transports
(`volc_signed_v4`, `aliyun_nls_ws`, …) which ship in subsequent SDK
releases.

## Security model

This SDK is open source and Go is reflective; the **leased upstream key
is observable** by anyone who can run a debugger against your process.
The platform mitigates this by:

- **Short lease TTLs** (default 15 min) so a stolen key has bounded
  utility.
- **Health-score feedback** — abuse patterns trigger automatic binding
  rotation on the platform side.
- **Revocable user keys** — revoke from the console and all leases tied
  to that key 401 within seconds.

What this SDK does at the "honest developer" tier:

- Lease auth_payload is kept only in process memory; never written to disk.
- Memory is best-effort overwritten on lease expiry / `Close`.
- 401 from upstream invalidates the cached lease so the next call refreshes.
- `Close` flushes outstanding usage reports and zeroes lease memory.

For stronger anti-reverse (e.g. a customer-distributed binary), layer
the SDK with `garble` and ship behind cgo. That is out of scope for
the open-source SDK.

## Configuration

| Field | Default | Notes |
|-------|---------|-------|
| `APIKey` | required | Your platform key (`sk-llmh-…`) |
| `BaseURL` | `https://api.llmhub.com` | Override for staging / on-prem |
| `HTTPClient` | `&http.Client{Timeout: 120s}` | For custom proxies / cert pinning |
| `LeaseLeadTime` | 60s | Refresh leases this far ahead of expiry |
| `ReportTimeout` | 10s | Per-event usage-report cap |
| `UserAgent` | `llmhub-go-sdk/<Version>` |  |

## Versioning

`v0.x.y` — pre-1.0; minor versions may include breaking changes until
the platform's SKU catalog stabilises. After v1.0 the SDK follows
strict semver against the public API surface.
