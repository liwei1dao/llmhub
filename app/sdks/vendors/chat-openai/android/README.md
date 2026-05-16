# LLMHub Chat SDK — Android

OpenAI-protocol chat completions, distributed as an Android Library
(`.aar`). One call to LLMHub mints a short-lived lease + real upstream
credential inside the SDK's native layer; your app talks directly to
the upstream provider through the SDK and never sees the credential.

- **Group / artifact**: `io.llmhub.sdk:llmhub-chat`
- **Version**: `0.1.0`
- **Min SDK**: 24 (Android 7.0)
- **ABIs shipped**: `arm64-v8a`, `armeabi-v7a`, `x86_64`
- **AAR size**: ~2.3 MB
- **Languages**: Kotlin (Java callable)

---

## 1. Install

### Option A — Maven (recommended)

```kotlin
// settings.gradle.kts
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://maven.llmhub.io/releases") }   // or your own mirror
    }
}

// app/build.gradle.kts
dependencies {
    implementation("io.llmhub.sdk:llmhub-chat:0.1.0")
}
```

The published POM pulls in `kotlinx-coroutines-android` and
`kotlinx-serialization-json` transitively — you do not need to add them.

### Option B — Local AAR

If you cannot use Maven, drop the AAR into `app/libs/`:

```kotlin
// app/build.gradle.kts
dependencies {
    implementation(files("libs/llmhub-chat-release.aar"))
    // bring deps in by hand — the AAR does NOT bundle them
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.1")
}
```

### ProGuard / R8

Nothing to add. The AAR ships a `consumer-rules.pro` that keeps the
JNI surface and `kotlinx.serialization` companions alive automatically.

---

## 2. Quick start

```kotlin
import io.llmhub.chat.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class ChatRepository(apiKey: String) : AutoCloseable {

    private val hub = LLMHubChat.create(
        baseUrl = "https://api.llmhub.io",
        apiKey  = apiKey,
    )

    suspend fun helloWorld(): String = withContext(Dispatchers.IO) {
        val session = hub.completions(skuId = "chat.gpt4o-class.standard")
        val resp = session.create(
            ChatCompletionRequest(
                messages = listOf(
                    ChatMessage.system("你是一个友好的助手"),
                    ChatMessage.user("用一句话介绍 LLMHub"),
                ),
                temperature = 0.7f,
            )
        )
        resp.choices.first().message.content
    }

    override fun close() = hub.close()
}
```

> The SDK is **blocking** at the JNI boundary. Always invoke `create`
> or collect `createStream(...)` from `Dispatchers.IO` (or an
> equivalent worker thread) — never from the main thread.

---

## 3. Authentication

Pass the user's LLMHub API key (`llmh-...`) to `LLMHubChat.create`.
The SDK uses it only to mint short-lived leases from
`https://<baseUrl>/sdk/credentials/issue`; the actual upstream
credential never crosses the JNI boundary into your code.

```kotlin
val hub = LLMHubChat.create(
    baseUrl = "https://api.llmhub.io",   // your LLMHub deployment
    apiKey  = BuildConfig.LLMHUB_API_KEY  // see "Secrets" below
)
```

### Secrets

Do not hard-code the API key in source. Use one of:

- Android `BuildConfig` driven by `gradle.properties` / `~/.gradle/gradle.properties`
- Encrypted SharedPreferences (`androidx.security:security-crypto`)
- Your auth backend issuing the key at runtime after user sign-in

---

## 4. Chat completions

### 4.1 Non-streaming

```kotlin
val session = hub.completions(skuId = "chat.gpt4o-class.standard")

val response: ChatCompletion = withContext(Dispatchers.IO) {
    session.create(
        ChatCompletionRequest(
            messages = listOf(
                ChatMessage.user("写一段 50 字以内的欢迎语")
            ),
            temperature = 0.7f,
            maxTokens   = 256,
        )
    )
}

println(response.choices.first().message.content)
println("用量：input=${response.usage.promptTokens}, output=${response.usage.completionTokens}")
```

### 4.2 Streaming

`createStream` returns a cold `Flow<ChatCompletionChunk>`. Cancelling
the collector (e.g. `coroutineScope.cancel()` or leaving the lifecycle
scope) cancels the underlying SSE stream and triggers a best-effort
usage report.

```kotlin
hub.completions(skuId = "chat.gpt4o-class.standard")
    .createStream(
        ChatCompletionRequest(
            messages = listOf(ChatMessage.user("讲个冷笑话")),
            temperature = 0.9f,
        )
    )
    .collect { chunk ->
        chunk.choices.firstOrNull()?.delta?.content?.let(::print)
    }
```

In Compose:

```kotlin
@Composable
fun ChatStream(hub: LLMHubChat, prompt: String) {
    val text = remember { mutableStateOf("") }
    LaunchedEffect(prompt) {
        text.value = ""
        hub.completions(skuId = "chat.gpt4o-class.standard")
            .createStream(ChatCompletionRequest(messages = listOf(ChatMessage.user(prompt))))
            .collect { chunk ->
                chunk.choices.firstOrNull()?.delta?.content?.let { text.value += it }
            }
    }
    Text(text.value)
}
```

### 4.3 Request shape

```kotlin
data class ChatCompletionRequest(
    val model: String = "",                  // 留空，SDK 用 lease 里的 upstream_model
    val messages: List<ChatMessage>,
    val temperature: Float? = null,
    val topP: Float? = null,
    val maxTokens: Int? = null,
    val stream: Boolean = false,             // SDK 内部覆盖；调用方无须设置
    val stop: List<String>? = null,
)
```

Roles: `ChatRole.SYSTEM / USER / ASSISTANT / TOOL`. Use the helpers
`ChatMessage.user / .system / .assistant` for the common cases.

---

## 5. Multimodal messages

`ChatMessage.content` is a sealed type:

```kotlin
sealed class MessageContent {
    data class Text(val text: String) : MessageContent()
    data class Parts(val parts: List<ContentPart>) : MessageContent()
}
```

Text-only messages keep the legacy string shape on the wire. Multimodal
messages serialise as an array of typed parts.

### 5.1 Vision (images)

`image_url` accepts either an `https://` URL or a `data:` URL with
base64 content:

```kotlin
val response = withContext(Dispatchers.IO) {
    hub.completions(skuId = "chat.gpt4o-vision.standard").create(
        ChatCompletionRequest(
            messages = listOf(
                ChatMessage.user(
                    ContentPart.TextPart("这张图里是什么？"),
                    ContentPart.Image(ImageUrl(
                        url = "https://example.com/photo.jpg",
                        detail = "high"   // "auto" / "low" / "high"
                    )),
                )
            )
        )
    )
}
```

Inline a local image as base64:

```kotlin
val bytes = context.contentResolver.openInputStream(uri)!!.use { it.readBytes() }
val b64   = android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP)
val dataUrl = "data:image/jpeg;base64,$b64"

ChatMessage.user(
    ContentPart.TextPart("描述一下"),
    ContentPart.Image(ImageUrl(url = dataUrl))
)
```

### 5.2 Audio input

```kotlin
ChatMessage.user(
    ContentPart.TextPart("把这段音频转成文字"),
    ContentPart.Audio(InputAudio(
        data   = base64FromWavFile,
        format = "wav"          // "wav" / "mp3"
    ))
)
```

The model your SKU resolves to must support audio inputs (e.g.
GPT-4o audio, Volc Doubao Omni). Check via `hub.listServices()`.

> 多模态尺寸建议：图片单边 ≤ 2048 px、base64 后 ≤ 5 MB；超过会触发上游 413。

---

## 6. Function calling (tools)

The full OpenAI tool-call loop is supported.

### 6.1 Define tools

```kotlin
import kotlinx.serialization.json.*

val weatherTool = Tool(
    function = FunctionDef(
        name        = "get_weather",
        description = "Look up the current weather for a city.",
        parameters  = buildJsonObject {
            put("type", "object")
            putJsonObject("properties") {
                putJsonObject("city") {
                    put("type", "string")
                    put("description", "City name, English or Chinese.")
                }
                putJsonObject("unit") {
                    put("type", "string")
                    put("enum", buildJsonArray {
                        add("celsius"); add("fahrenheit")
                    })
                }
            }
            put("required", buildJsonArray { add("city") })
        }
    )
)
```

### 6.2 Single round-trip

```kotlin
val first: ChatCompletion = withContext(Dispatchers.IO) {
    hub.completions(skuId).create(
        ChatCompletionRequest(
            messages = mutableListOf(ChatMessage.user("北京今天多少度？")),
            tools = listOf(weatherTool),
        )
    )
}

val msg = first.choices.first().message
val calls = msg.toolCalls.orEmpty()
if (calls.isEmpty()) {
    // model answered directly
    return msg.content
}

// Execute each tool call and feed the results back
val followUp = mutableListOf<ChatMessage>().apply {
    add(ChatMessage.user("北京今天多少度？"))
    add(msg)                                     // assistant's tool_calls message
    calls.forEach { call ->
        val args = Json.parseToJsonElement(call.function.arguments).jsonObject
        val city = args["city"]!!.jsonPrimitive.content
        val unit = args["unit"]?.jsonPrimitive?.content ?: "celsius"
        val result = weatherService.lookup(city, unit)         // your own impl
        add(ChatMessage.tool(toolCallId = call.id, result = result))
    }
}

val second = withContext(Dispatchers.IO) {
    hub.completions(skuId).create(
        ChatCompletionRequest(messages = followUp, tools = listOf(weatherTool))
    )
}
println(second.choices.first().message.content)
```

### 6.3 Streaming tool calls

In stream mode the `toolCalls` array on `ChatDelta` carries **partial**
function calls — `index` identifies the slot, `function.arguments` is
emitted as a sequence of string slices that must be concatenated.

```kotlin
// Per-index accumulator
val calls = mutableMapOf<Int, ToolCallBuilder>()
data class ToolCallBuilder(
    var id: String = "",
    var name: String = "",
    val args: StringBuilder = StringBuilder(),
)

hub.completions(skuId).createStream(
    ChatCompletionRequest(messages = listOf(ChatMessage.user("北京今天多少度？")),
                          tools = listOf(weatherTool))
).collect { chunk ->
    val ch = chunk.choices.firstOrNull() ?: return@collect
    ch.delta.content?.let(::print)                         // free-form text
    ch.delta.toolCalls?.forEach { d ->
        val slot = calls.getOrPut(d.index) { ToolCallBuilder() }
        d.id?.let               { slot.id = it }
        d.function?.name?.let   { slot.name = it }
        d.function?.arguments?.let { slot.args.append(it) }
    }
    if (ch.finishReason == "tool_calls") {
        // assemble final ToolCall list, run them, build follow-up turn …
    }
}
```

### 6.4 `tool_choice`

Forwarded raw to the upstream. Common shapes:

```kotlin
import kotlinx.serialization.json.*

// Let the model decide (default)
ChatCompletionRequest(/* … */, toolChoice = JsonPrimitive("auto"))
// Force a tool call
ChatCompletionRequest(/* … */, toolChoice = JsonPrimitive("required"))
// Disable tools for this turn even though they're defined
ChatCompletionRequest(/* … */, toolChoice = JsonPrimitive("none"))
// Force a specific function
ChatCompletionRequest(
    /* … */,
    toolChoice = buildJsonObject {
        put("type", "function")
        putJsonObject("function") { put("name", "get_weather") }
    }
)
```

### 6.5 Structured outputs (JSON mode)

```kotlin
ChatCompletionRequest(
    messages = listOf(ChatMessage.user("从这段文字里抽出公司名和创立年份，用 JSON 返回。" /* … */)),
    responseFormat = buildJsonObject { put("type", "json_object") }
)
```

Or full JSON-schema-constrained:

```kotlin
responseFormat = buildJsonObject {
    put("type", "json_schema")
    putJsonObject("json_schema") {
        put("name", "company_extract")
        putJsonObject("schema") { /* JSON Schema */ }
        put("strict", true)
    }
}
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
  `ChatCompletionRequest.tools`. When the model emits a `toolCalls`,
  route the call back to the MCP server, then feed the result in as
  a `role=tool` message. The SDK doesn't need to know MCP exists —
  it just transports the tool definitions and the round-trip messages.

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

Query what the authenticated user is subscribed to:

```kotlin
val services: List<ServiceEntry> = withContext(Dispatchers.IO) {
    hub.listServices()
}
services.forEach { svc ->
    Log.d("LLMHub", "${svc.skuId}: ${svc.quotaRemaining}/${svc.quotaTotal} left, qps=${svc.qpsLimit}")
}
```

Use this at app startup to decide which SKU IDs to show in your UI.

---

## 9. Errors

Every failure surfaces as `LLMHubException(code, message)`. Map your
UI strings off **`code`** (wire-stable), not `message` (human, may
change).

```kotlin
try {
    session.create(request)
} catch (e: LLMHubException) {
    when (e.code) {
        "unauthorized"         -> reLogin()
        "quota_exceeded"       -> showUpgradeDialog()
        "not_subscribed"       -> showSubscriptionScreen()
        "no_binding_available" -> showOutageNotice()
        "rate_limited"         -> backOffAndRetry()
        "network_error",
        "timeout"              -> showOfflineBanner()
        else                   -> showGenericError(e.message)
    }
}
```

| code                   | What it means                                          |
| ---------------------- | ------------------------------------------------------ |
| `missing_api_key`      | `apiKey` empty                                         |
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
| `cancelled`            | stream collector cancelled                             |

---

## 10. Lifecycle

| Type                  | Lifetime                                  |
| --------------------- | ----------------------------------------- |
| `LLMHubChat`          | App-scoped. Create once, share via DI.    |
| `LLMHubChat.ChatSession` | Per-call cheap. Don't cache across SKUs. |

Both implement `AutoCloseable`. Call `.close()` when you're sure no
more requests will run — typically tied to your `Application.onTerminate`
or the lifecycle of your auth scope. Forgetting to close leaks a
native handle until the process exits, which is benign in most apps.

```kotlin
// Hilt / Koin module
@Provides @Singleton
fun provideLLMHub(): LLMHubChat =
    LLMHubChat.create("https://api.llmhub.io", BuildConfig.LLMHUB_API_KEY)
```

### Threading

- Every public method blocks the calling thread on the JNI boundary.
- `createStream(...)` already wraps its driver in `Dispatchers.IO` via
  `flowOn(Dispatchers.IO)`; you can `collect` from a UI scope.
- For the non-streaming `create(...)`, dispatch yourself
  (`withContext(Dispatchers.IO) { ... }`).

---

## 11. Logging & observability

The SDK logs nothing by default. Errors are returned as exceptions —
log them in your own `catch`.

Usage is reported automatically: on every call the SDK posts the
outcome (success / rate_limited / auth_failed / timeout / upstream_error)
plus latency to `POST /sdk/usage/report`. There is nothing for you to
do on the consumer side.

---

## 12. Permissions

The library ships with a single permission:

```xml
<uses-permission android:name="android.permission.INTERNET" />
```

It is merged automatically into your app's manifest by the Android
Gradle Plugin.

---

## 13. FAQ

**Q: Can I use the SDK from Java instead of Kotlin?**
Yes. The public API is annotated `@JvmStatic` where it matters; data
classes have standard getters. Streaming requires a coroutine bridge
(use `kotlinx-coroutines-rx3` or `kotlinx-coroutines-reactive` adaptors).

**Q: Why isn't `model` set?**
You pass a *sku* (e.g. `chat.gpt4o-class.standard`), not a model name.
The platform decides which upstream model to route to, returns it
via the lease's `upstream_model`, and the SDK populates the field
for you. Setting `model` explicitly overrides this.

**Q: Where does my key go on the wire?**
The LLMHub key is sent only to `https://<baseUrl>/sdk/credentials/issue`
(over TLS) to mint a lease. The upstream provider's API key — which
the lease contains — is then sent only to the upstream's endpoint
(also TLS). At no point does the upstream key leave the SDK's native
memory into Kotlin land.

**Q: Can I extract the upstream key by reverse-engineering the AAR?**
Not without significant effort. The Rust core is compiled, stripped,
LTO'd, and link-time-folded; endpoint paths are XOR-encrypted with a
per-build key (litcrypt2); Kotlin glue is R8-minified. Resourceful
attackers can still get it eventually, but the bar is raised well
above "casual abuse".

---

## License

Proprietary · © 2026 LLMHub. Contact `dev@llmhub.io` for licensing.
