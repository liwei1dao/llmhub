# LLMHub Chat SDK — iOS / macOS

OpenAI-protocol chat completions, shipped as a **binary-only** Swift
Package (`LLMHubChatKit.xcframework`). The Rust core + Swift facade
are both pre-compiled — your app gets a single `.xcframework`,
imports it via SPM, and calls a friendly Swift API.

- **Package**: `LLMHubChatKit`
- **Public type**: `LLMHubChat` (and friends — see below)
- **Version**: `0.1.0`
- **Min OS**: iOS 15.0 / macOS 12.0
- **Slices shipped**: `ios-arm64` (device), `ios-arm64_x86_64-simulator`
- **Module name ≠ main class name** — `import LLMHubChatKit`, then use `LLMHubChat`
- **Languages**: Swift 5.9+

---

## 1. Install

### Swift Package Manager (Xcode)

`File → Add Package Dependencies… → Enter Package URL`:

```
https://github.com/llmhub/llmhub-chat-ios.git
```

Pin to `Up to Next Major Version: 0.1.0`.

### Swift Package Manager (Package.swift)

```swift
let package = Package(
    name: "MyApp",
    dependencies: [
        .package(url: "https://github.com/llmhub/llmhub-chat-ios.git", from: "0.1.0"),
    ],
    targets: [
        .target(name: "MyApp", dependencies: [
            .product(name: "LLMHubChatKit", package: "llmhub-chat-ios"),
        ]),
    ]
)
```

### Local path (private distribution)

```swift
.package(path: "/path/to/app/sdks/vendors/chat-openai/ios/LLMHubChatKit"),
```

---

## 2. Quick start

```swift
import LLMHubChatKit                              // ← module is "Kit"
                                                  // ← type is friendly
import Foundation

@MainActor
final class ChatViewModel: ObservableObject {

    private let hub: LLMHubChat

    init(apiKey: String) throws {
        hub = try LLMHubChat(
            baseURL: "https://api.llmhub.io",
            apiKey:  apiKey
        )
    }

    func helloWorld() async throws -> String {
        let chat = try hub.completions(skuId: "chat.gpt4o-class.standard")
        let reply = try await Task.detached { [chat] in
            try chat.create(.init(
                messages: [
                    .system("你是一个友好的助手"),
                    .user("用一句话介绍 LLMHub"),
                ],
                temperature: 0.7
            ))
        }.value
        return reply.choices.first?.message.content ?? ""
    }
}
```

> Calls are **synchronous** on the C-ABI boundary. Always wrap them in
> `Task.detached`, a background queue, or a `withCheckedThrowingContinuation`
> off the main thread — never block the main actor.

---

## 3. Authentication

Pass the user's LLMHub API key (`llmh-...`) to `LLMHubChat.init`. The
SDK uses it only to mint short-lived leases; the upstream provider's
credential never crosses the C ABI back into your Swift code.

```swift
let hub = try LLMHubChat(
    baseURL: "https://api.llmhub.io",
    apiKey:  ProcessInfo.processInfo.environment["LLMHUB_API_KEY"]!
)
```

### Secrets

Do not hard-code the API key. Use one of:

- **Keychain** (`Security.framework`) seeded by your auth backend at sign-in.
- An `xcconfig` driving `Info.plist` for development only — **never ship it**.
- A backend endpoint returning the key after user auth.

---

## 4. Chat completions

### 4.1 Non-streaming

```swift
let chat = try hub.completions(skuId: "chat.gpt4o-class.standard")

let response: ChatCompletion = try await Task.detached { [chat] in
    try chat.create(.init(
        messages: [.user("写一段 50 字以内的欢迎语")],
        temperature: 0.7,
        maxTokens: 256
    ))
}.value

print(response.choices.first?.message.content ?? "")
print("用量：input=\(response.usage.promptTokens), output=\(response.usage.completionTokens)")
```

### 4.2 Streaming

`createStream(_:)` returns an `AsyncThrowingStream<ChatCompletionChunk, Error>`.
Cancelling the surrounding `Task` cancels the underlying SSE stream and
triggers a best-effort usage report on the platform.

```swift
let chat = try hub.completions(skuId: "chat.gpt4o-class.standard")

let task = Task {
    do {
        for try await chunk in chat.createStream(.init(
            messages: [.user("讲个冷笑话")],
            temperature: 0.9
        )) {
            if let s = chunk.choices.first?.delta.content {
                print(s, terminator: "")
            }
        }
    } catch {
        print("stream failed:", error)
    }
}

// Later — cancel from anywhere:
task.cancel()
```

In SwiftUI:

```swift
struct ChatView: View {
    let hub: LLMHubChat
    @State private var text = ""

    var body: some View {
        Text(text)
            .task {
                guard let chat = try? hub.completions(skuId: "chat.gpt4o-class.standard") else { return }
                do {
                    for try await chunk in chat.createStream(.init(messages: [.user("hi")])) {
                        text += chunk.choices.first?.delta.content ?? ""
                    }
                } catch {
                    text = "error: \(error)"
                }
            }
    }
}
```

### 4.3 Request shape

```swift
public struct ChatCompletionRequest: Codable, Sendable {
    public var model: String              // 留空，SDK 会用 lease 里的 upstream_model
    public var messages: [ChatMessage]
    public var temperature: Float?
    public var topP: Float?
    public var maxTokens: Int?
    public var stream: Bool               // SDK 内部覆盖；无须设置
    public var stop: [String]?
}
```

Roles: `ChatRole.system / .user / .assistant / .tool`. Convenience
factories: `ChatMessage.user(_:)`, `.system(_:)`, `.assistant(_:)`.

---

## 5. Multimodal messages

`ChatMessage.content` is a polymorphic enum:

```swift
public enum MessageContent: Codable, Sendable {
    case text(String)
    case parts([ContentPart])
}
```

Text-only messages keep the legacy string shape on the wire. Multimodal
messages serialise as an array of typed parts.

### 5.1 Vision (images)

`image_url` accepts either an `https://` URL or a `data:` URL with
base64 content:

```swift
let chat = try hub.completions(skuId: "chat.gpt4o-vision.standard")
let r = try await Task.detached { [chat] in
    try chat.create(.init(messages: [
        ChatMessage.user(
            .text("这张图里是什么？"),
            .imageURL(.init(url: "https://example.com/photo.jpg",
                            detail: "high"))   // "auto" / "low" / "high"
        )
    ]))
}.value
```

Inline a local image as base64:

```swift
let bytes = try Data(contentsOf: localURL)
let b64 = bytes.base64EncodedString()
let dataURL = "data:image/jpeg;base64,\(b64)"

let msg = ChatMessage.user(
    .text("描述一下"),
    .imageURL(.init(url: dataURL))
)
```

### 5.2 Audio input

```swift
let pcm = try Data(contentsOf: wavURL)
let audioMsg = ChatMessage.user(
    .text("把这段音频转成文字"),
    .inputAudio(.init(data: pcm.base64EncodedString(), format: "wav"))   // "wav" / "mp3"
)
```

The model behind your SKU must support audio inputs (e.g. GPT-4o audio,
Volc Doubao Omni). Check via `hub.listServices()`.

> 多模态尺寸建议：图片单边 ≤ 2048 px、base64 后 ≤ 5 MB；超过会触发上游 413。

---

## 6. Function calling (tools)

The full OpenAI tool-call loop is supported.

### 6.1 Define tools

`FunctionDef.parameters` is a `JSONValue` — build the schema as raw JSON:

```swift
let weatherTool = Tool(function: .init(
    name: "get_weather",
    description: "Look up the current weather for a city.",
    parameters: .object([
        "type": .string("object"),
        "properties": .object([
            "city": .object([
                "type": .string("string"),
                "description": .string("City name, English or Chinese."),
            ]),
            "unit": .object([
                "type": .string("string"),
                "enum": .array([.string("celsius"), .string("fahrenheit")]),
            ]),
        ]),
        "required": .array([.string("city")]),
    ])
))
```

### 6.2 Single round-trip

```swift
let chat = try hub.completions(skuId: skuId)

let first = try await Task.detached { [chat] in
    try chat.create(.init(
        messages: [.user("北京今天多少度？")],
        tools: [weatherTool]
    ))
}.value

let msg = first.choices.first!.message
let calls = msg.toolCalls ?? []
if calls.isEmpty {
    return msg.content   // model answered directly
}

// Execute each tool call and feed the results back
var followUp: [ChatMessage] = [.user("北京今天多少度？"), msg]
for call in calls {
    let argsData = call.function.arguments.data(using: .utf8) ?? Data()
    let args = (try? JSONSerialization.jsonObject(with: argsData)) as? [String: Any] ?? [:]
    let city = args["city"] as? String ?? ""
    let unit = args["unit"] as? String ?? "celsius"
    let result = try await weatherService.lookup(city, unit)        // your own impl
    followUp.append(.tool(callId: call.id, result: result))
}

let second = try await Task.detached { [chat] in
    try chat.create(.init(messages: followUp, tools: [weatherTool]))
}.value
print(second.choices.first?.message.content ?? "")
```

### 6.3 Streaming tool calls

In stream mode the `toolCalls` array on `ChatDelta` carries **partial**
function calls — `index` identifies the slot, `function.arguments` is
emitted as a sequence of string slices that must be concatenated.

```swift
struct ToolCallBuilder { var id = ""; var name = ""; var args = "" }
var slots: [Int: ToolCallBuilder] = [:]

let stream = chat.createStream(.init(
    messages: [.user("北京今天多少度？")],
    tools: [weatherTool]
))

for try await chunk in stream {
    guard let ch = chunk.choices.first else { continue }
    if let s = ch.delta.content { print(s, terminator: "") }
    for d in ch.delta.toolCalls ?? [] {
        var slot = slots[d.index] ?? ToolCallBuilder()
        if let id = d.id           { slot.id = id }
        if let n  = d.function?.name      { slot.name = n }
        if let a  = d.function?.arguments { slot.args += a }
        slots[d.index] = slot
    }
    if ch.finishReason == "tool_calls" {
        // assemble final ToolCall list, run them, build follow-up turn …
    }
}
```

### 6.4 `tool_choice`

```swift
// Let the model decide (default)
.init(messages: msgs, tools: tools, toolChoice: .auto)
// Force a tool call
.init(messages: msgs, tools: tools, toolChoice: .required)
// Disable tools for this turn even though they're defined
.init(messages: msgs, tools: tools, toolChoice: .none)
// Force a specific function
.init(messages: msgs, tools: tools, toolChoice: .function(name: "get_weather"))
```

### 6.5 Structured outputs (JSON mode)

```swift
.init(
    messages: [.user("从这段文字里抽出公司名和创立年份，用 JSON 返回。…")],
    responseFormat: .object(["type": .string("json_object")])
)
```

Or full JSON-schema-constrained:

```swift
.init(
    messages: msgs,
    responseFormat: .object([
        "type": .string("json_schema"),
        "json_schema": .object([
            "name": .string("company_extract"),
            "schema": .object([/* JSON Schema */]),
            "strict": .bool(true),
        ])
    ])
)
```

---

## 7. MCP & Skills — how to fit them in

The SDK is intentionally **wire-level**: it speaks OpenAI
`/v1/chat/completions`. MCP and Skills aren't wire features — they're
*composition patterns above tool calling*. Use them like this:

- **MCP (Model Context Protocol)** — your app connects to an MCP
  server and enumerates its tools (`list_tools` RPC). Convert each
  MCP tool to a `Tool` (its JSON Schema goes straight into
  `FunctionDef.parameters`) and pass them to
  `ChatCompletionRequest.tools`. When the model emits `toolCalls`,
  route each call back to the MCP server, then feed the result in as
  a `role=.tool` message. The SDK doesn't need to know MCP exists —
  it just transports tool definitions and round-trip messages.

- **Skills** — a "skill" is typically a bundle of `(system prompt,
  tool list, optional resources)`. Compose it at app level: pick the
  right system prompt + register the right `Tool` set per skill.
  Switching skill ≡ swapping the prompt + tools in the request.

If/when LLMHub's platform side adds native MCP-server proxying or
hosted Skills, the wire shape will gain new fields; the SDK exposes
them via `responseFormat`-style raw-JSON pass-throughs without a
breaking change.

---

## 8. Service discovery

```swift
let services: [ServiceEntry] = try await Task.detached { [hub] in
    try hub.listServices()
}.value

for svc in services {
    print("\(svc.skuId): \(svc.quotaRemaining)/\(svc.quotaTotal) left, qps=\(svc.qpsLimit)")
}
```

Use this at app startup to decide which SKU IDs to show in your UI.

---

## 9. Errors

Every failure surfaces as `LLMHubError(code:message:)`. Switch on
**`code`** (wire-stable), not `message` (human, may change).

```swift
do {
    _ = try chat.create(request)
} catch let e as LLMHubError {
    switch e.code {
    case "unauthorized":         await reSignIn()
    case "quota_exceeded":       showUpgradeSheet()
    case "not_subscribed":       showSubscriptionScreen()
    case "no_binding_available": showOutageNotice()
    case "rate_limited":         await backOffAndRetry()
    case "network_error",
         "timeout":              showOfflineBanner()
    default:                     showGenericError(e.message)
    }
}
```

| code                   | What it means                                          |
| ---------------------- | ------------------------------------------------------ |
| `invalid_argument`     | `baseURL` / `apiKey` / `skuId` empty                   |
| `init_failed`          | native handle could not be created                     |
| `unauthorized`         | API key rejected by the platform                       |
| `sku_not_found`        | sku doesn't exist                                      |
| `sku_deprecated`       | sku exists but no longer active                        |
| `not_subscribed`       | user has no active subscription for the sku            |
| `quota_exceeded`       | monthly / daily quota exhausted                        |
| `no_binding_available` | no healthy upstream account in the pool                |
| `lease_expired`        | rare; SDK auto-refreshes, but force-refresh on retry   |
| `upstream_error`       | upstream provider returned a non-2xx                   |
| `network_error`        | DNS / TCP / TLS / read failure                         |
| `decode_error`         | upstream returned malformed JSON                       |
| `cancelled`            | stream task cancelled                                  |

---

## 10. Lifecycle

| Type                       | Lifetime                                  |
| -------------------------- | ----------------------------------------- |
| `LLMHubChat`               | App-scoped. Create once, share via DI / `@Observable`. |
| `LLMHubChat.ChatSession`   | Per-call cheap. Don't cache across SKUs.  |

Native handles are released automatically in `deinit` — no manual
`close()` to call. Hold a strong reference to `LLMHubChat` for as long
as you might issue a request.

```swift
// SwiftUI App
@main
struct MyApp: App {
    @StateObject private var llmhub = LLMHubHolder()

    var body: some Scene {
        WindowGroup { RootView().environmentObject(llmhub) }
    }
}

@MainActor
final class LLMHubHolder: ObservableObject {
    let hub: LLMHubChat
    init() {
        hub = try! LLMHubChat(baseURL: "https://api.llmhub.io",
                              apiKey: KeychainStore.apiKey())
    }
}
```

### Threading

- `create(_:)` and `listServices()` block the calling thread on the
  C-ABI call. Wrap in `Task.detached` to leave the main actor.
- `createStream(_:)` already dispatches its driver to a detached task;
  you can `for try await` from `@MainActor`.

---

## 11. Public API surface

The XCFramework ships a `.swiftinterface` that exposes only signatures
(no implementation). The complete public surface is:

```swift
public final class LLMHubChat: @unchecked Sendable {
    public init(baseURL: String, apiKey: String) throws
    public func completions(skuId: String) throws -> ChatSession
    public func listServices() throws -> [ServiceEntry]

    public final class ChatSession: @unchecked Sendable {
        public func create(_ request: ChatCompletionRequest) throws -> ChatCompletion
        public func createStream(_ request: ChatCompletionRequest)
            -> AsyncThrowingStream<ChatCompletionChunk, Error>
    }
}

public struct LLMHubError: Error, Sendable {
    public let code: String
    public let message: String
}

public enum ChatRole: String, Codable, Sendable { case system, user, assistant, tool }

public enum MessageContent: Codable, Sendable { case text(String); case parts([ContentPart]) }

public enum ContentPart: Codable, Sendable {
    case text(String)
    case imageURL(ImageURL)
    case inputAudio(InputAudio)
}
public struct ImageURL: Codable, Sendable    { public var url: String; public var detail: String? }
public struct InputAudio: Codable, Sendable  { public var data: String; public var format: String }

public enum JSONValue: Codable, Sendable, Hashable {
    case null, bool(Bool), int(Int64), double(Double), string(String),
         array([JSONValue]), object([String: JSONValue])
}

public struct Tool: Codable, Sendable        { public var kind: String; public var function: FunctionDef }
public struct FunctionDef: Codable, Sendable { public var name: String; public var description: String?; public var parameters: JSONValue }
public enum ToolChoice: Codable, Sendable    { case auto, none, required; case function(name: String) }
public struct ToolCall: Codable, Sendable    { public var id: String; public var kind: String; public var function: ToolFunctionCall }
public struct ToolFunctionCall: Codable, Sendable { public var name: String; public var arguments: String }

public struct ChatMessage: Codable, Sendable {
    public var role: ChatRole
    public var content: MessageContent
    public var name: String?
    public var toolCalls: [ToolCall]?
    public var toolCallId: String?
    public static func user(_ s: String) -> ChatMessage
    public static func system(_ s: String) -> ChatMessage
    public static func assistant(_ s: String) -> ChatMessage
    public static func user(_ parts: ContentPart...) -> ChatMessage
    public static func assistant(toolCalls: [ToolCall]) -> ChatMessage
    public static func tool(callId: String, result: String) -> ChatMessage
}

public struct ChatCompletionRequest: Codable, Sendable {
    public var model: String
    public var messages: [ChatMessage]
    public var temperature: Float?
    public var topP: Float?
    public var maxTokens: Int?
    public var stream: Bool
    public var stop: [String]?
    public var tools: [Tool]?
    public var toolChoice: ToolChoice?
    public var parallelToolCalls: Bool?
    public var responseFormat: JSONValue?
}

public struct ChatCompletion: Codable, Sendable        { /* id, model, choices, usage */ }
public struct ChatCompletionChunk: Codable, Sendable   { /* id, model, choices, usage? */ }
public struct ChatCompletionChoice: Codable, Sendable  { /* index, message, finishReason? */ }
public struct ChatChunkChoice: Codable, Sendable       { /* index, delta, finishReason? */ }
public struct ChatDelta: Codable, Sendable             { public var role: String?; public var content: String?; public var toolCalls: [ToolCallDelta]? }
public struct ToolCallDelta: Codable, Sendable         { public var index: Int; public var id: String?; public var kind: String?; public var function: ToolFunctionCallDelta? }
public struct ToolFunctionCallDelta: Codable, Sendable { public var name: String?; public var arguments: String? }
public struct Usage: Codable, Sendable                 { /* promptTokens, completionTokens, totalTokens */ }
public struct ServiceEntry: Codable, Sendable          { /* skuId, quota*, qpsLimit, … */ }
```

---

## 12. FAQ

**Q: Why isn't `model` set?**
You pass a *sku* (e.g. `chat.gpt4o-class.standard`), not a model name.
The platform picks the upstream model, returns it via the lease's
`upstream_model`, and the SDK fills the field. Setting `model`
explicitly overrides this.

**Q: Where does my key go on the wire?**
The LLMHub key is sent only to `https://<baseURL>/sdk/credentials/issue`
(TLS) to mint a lease. The upstream provider's API key — which the
lease contains — is sent only to the upstream's endpoint (TLS). The
upstream key never crosses the C ABI back into your Swift code.

**Q: Can I extract the upstream key by reverse-engineering the .xcframework?**
Not without significant effort. The Rust core is compiled, stripped,
and LTO'd; endpoint paths are XOR-encrypted with a per-build key; the
Swift facade is `-enable-library-evolution`-compiled (no source ships).
Resourceful attackers can still get it eventually, but the bar is
raised well above "casual abuse".

**Q: Why the `Kit` suffix on the module name?**
Swift's library-evolution mode (required for binary distribution) gets
confused if the module name equals the name of a top-level type:
`LLMHubChat.ChatRole` becomes ambiguous between "module's ChatRole"
and "class LLMHubChat's nested ChatRole". Apple's frameworks
side-step this by always using `UIKit / UIView`, `MapKit / MKMapView`,
etc. We follow the same pattern: module `LLMHubChatKit`, type `LLMHubChat`.

**Q: Does this support Catalyst / macOS / tvOS / visionOS?**
The current XCFramework ships iOS device + iOS simulator slices only.
Other Apple platforms are buildable from the same Rust + Swift sources
— ask if you need them.

---

## License

Proprietary · © 2026 LLMHub. Contact `dev@llmhub.io` for licensing.
