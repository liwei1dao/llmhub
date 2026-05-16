# LLMHub SDKs

LLMHub 是聚合 **SDK**（不是网关）—— 平台不在用户调用的关键路径上。
SDK 在用户进程内完成：

1. 用 `api_key` 向 LLMHub 平台申请 **短时 lease**（带真实上游凭据）
2. 直接打到上游厂商（OpenAI / 火山 ARK / DeepSeek …）
3. 回头向平台上报用量

为了让"真实上游凭据"永远不暴露给宿主 App，SDK 的核心必须是
**编译后的原生库 + 混淆 / 字符串加密**。这个目录就是核心库 + 各平
台封装层的家，按 `服务/平台` 双层归属。

## 目录结构

```
app/sdks/
├── Cargo.toml                     # 单一 Rust workspace（hard release profile）
├── rust-toolchain.toml            # 钉死 1.86.0 + android/ios target
├── .cargo/config.toml             # 链接器加固
├── README.md
├── .gitignore
│
├── core/                          # 跨厂商共享 Rust 代码
│   └── llmhub-core/               # lease / 用量上报 / HTTP transport
│
└── vendors/                       # 每个服务一个目录，按厂商/协议归属
    └── chat-openai/               # 服务：聊天大模型（OpenAI 协议）
        ├── core/                  # 该服务的所有内部源码（不对外）
        │   ├── capability/        # llmhub-chat-openai：Rust 能力 crate
        │   ├── ffi-android/       # JNI cdylib (Rust)
        │   ├── ffi-ios/           # C-ABI staticlib (Rust)
        │   ├── facade-ios/        # Swift facade 源码（编译进 xcframework）
        │   │   └── Sources/LLMHubChat.swift
        │   └── scripts/
        │       ├── build-android.sh   # cargo-ndk × {arm64-v8a, armv7, x86_64}
        │       └── build-ios.sh       # cargo + cbindgen + swiftc + libtool + xcframework
        │
        ├── android/               # Android AAR 项目（对外产物）
        │   ├── settings.gradle.kts
        │   ├── build.gradle.kts
        │   ├── gradle.properties
        │   ├── gradle/wrapper/    # Gradle 8.11.1
        │   └── llmhub-chat/       # library module
        │       ├── build.gradle.kts
        │       ├── consumer-rules.pro
        │       ├── proguard-rules.pro
        │       └── src/main/
        │           ├── AndroidManifest.xml
        │           ├── kotlin/io/llmhub/chat/      # public facade（编进 classes.jar）
        │           └── jniLibs/                    # 由 build-android.sh 填充
        │
        └── ios/                   # iOS Swift Package（对外产物，全二进制）
            └── LLMHubChatKit/
                ├── Package.swift                   # 只声明 binaryTarget
                └── LLMHubChatKit.xcframework/      # 由 build-ios.sh 产出
                    ├── ios-arm64/
                    │   └── LLMHubChatKit.framework/
                    │       ├── LLMHubChatKit       # 合并后的 .a (Rust + Swift)
                    │       └── Modules/LLMHubChatKit.swiftmodule/
                    │           ├── *.swiftmodule   # 二进制编译产物
                    │           └── *.swiftinterface # public API 签名（ABI-stable）
                    └── ios-arm64_x86_64-simulator/...
```

> iOS 侧的"对外不可读"策略：
> - **Rust 实现**：编进 `.a`，符号 strip。
> - **Swift facade 实现**：编成 `.swiftmodule`（二进制 mangled）+ 静态归并进同一个 `.a`。
> - **公共 API 签名**：泄露到 `.swiftinterface` 里 —— 这是 Swift ABI 稳定性的设计前提（等价于
>   `.h` 文件），所有商业 SDK 都长这样，无法回避。但里面只有"这个类有哪些公开方法"，没有
>   "方法怎么实现"。

### 为什么按 `core / vendors/<服务>/平台` 组织

`vendors/<服务>/{core,android,ios}` 比 `core/, platforms/<plat>/<svc>/`
更易管理：

* 看一眼 `vendors/chat-openai/` 就知道这个服务有哪些平台版本。
* 删一个服务只删一个目录，不会留下分散在多个 platform 目录的孤立子目录。
* 多服务上线时（tts、asr、…），同一个 PR 的改动天然聚在一起。

跨厂商真正共享的代码（lease 流程、HTTP transport、用量上报）放在
顶层 `core/llmhub-core/`，被每个服务的内层 `core/` 通过 workspace
path 依赖。两个 `core/` 在不同层级，含义清晰：
顶层 = "跨服务内核"，服务内 = "该服务的 Rust 实现"。

各服务的 ffi-android cdylib 是独立的，所以多服务并存时每个
都有自己的 `.so`（共享部分静态链入各 `.so`，加起来比一个大胖 .so
要费一点空间，但解耦的边界换来这点开销值得）。

### 增加新服务（如 TTS）

```
vendors/
├── chat-openai/  ← 现有
└── tts-openai/   ← 新增，目录结构完全一致
    ├── core/{capability, ffi-android, ffi-ios, scripts}
    ├── android/                # 独立 AAR
    └── ios/                    # 独立 SwiftPM
```

记得：
1. 在根 [Cargo.toml](Cargo.toml) 的 `members` 里加新 crate 路径。
2. 如果用了同一个协议家族（OpenAI 兼容），capability crate 可以
   复用一些 helper，但每个服务还是一个独立 crate，避免编译期循环。
3. 每个服务自己的 ffi-android crate 把 `[lib].name` 写成短名，
   Kotlin `System.loadLibrary("<short>")` 就能加载，
   不必拼出 `llmhub-tts-openai-ffi-android` 这种长串。

## 安全模型

| 资产                     | 谁能看到                                |
| ----------------------- | --------------------------------------- |
| LLMHub `api_key`         | 用户、SDK 公共 API、平台                  |
| Lease（含上游凭据）         | **只在 Rust 内存里**，不返回到 Kotlin / Swift |
| 上游 `Authorization`      | **只在 Rust 内存里 + TLS 出口**            |

防御链：

1. **lease 在 Rust 进程内驻留**：JNI / C-ABI 边界从来不返回
   `auth_payload`。Kotlin / Swift 拿到的是上游已经响应完的 JSON。
2. **编译加固**（[Cargo.toml](Cargo.toml) 的 `[profile.release]`）：
   `lto=fat`、`codegen-units=1`、`panic=abort`、`strip=symbols`、
   `opt-level=z`。
3. **字符串加密**：`litcrypt2` 把"/sdk/credentials/issue"、
   "Authorization"、"Bearer " 等高识别度常量在 .so / .a 里 XOR 加密，
   运行时才解密。编译时通过 `LITCRYPT_ENCRYPT_KEY` 注入种子
   （每次 release 不同）。
4. **链接器加固**（仅 Android，见 [.cargo/config.toml](.cargo/config.toml)）：
   `--gc-sections --icf=all -z relro -z now --exclude-libs,ALL`。
5. **R8 / ProGuard**：
   [consumer-rules.pro](vendors/chat-openai/android/llmhub-chat/consumer-rules.pro)
   只保留 JNI 必须的符号，其余 Kotlin 代码被混淆。
6. **Swift `@_implementationOnly`**：iOS 侧的 C 模块只在编译期可见，
   `LLMHubChatKit.xcframework` 的 `.swiftinterface` 不会泄漏任何 C 符号。
7. **全二进制 iOS 发布**：Swift facade 被 swiftc 编进 `.swiftmodule`
   （mangled binary），并 `libtool -static` 进 framework 的单一 .a 里；
   消费侧只看得到 `.swiftinterface` 这一份公共 API 签名（等价 .h），
   实现完全不可读。

## 构建

### 一次性环境准备

```bash
# Rust 1.86 + 所有 target
rustup toolchain install 1.86.0
rustup target add aarch64-linux-android armv7-linux-androideabi x86_64-linux-android \
                  aarch64-apple-ios aarch64-apple-ios-sim x86_64-apple-ios

# 跨编译 / 头文件生成工具
cargo install cargo-ndk cbindgen

# Android NDK（Android Studio → SDK Manager → NDK (Side by side)）
export ANDROID_NDK_HOME=/path/to/ndk/26.x.x

# JDK 17（Gradle 8.11.1 / AGP 8.5 都要 17，别用 JDK 21+）
export JAVA_HOME=/path/to/jdk-17
```

### Android AAR

```bash
cd app/sdks/vendors/chat-openai/android
./gradlew :llmhub-chat:assembleRelease
# 产物: llmhub-chat/build/outputs/aar/llmhub-chat-release.aar (~2.3 MB)
# 内含 jni/{arm64-v8a, armeabi-v7a, x86_64}/libllmhub_chat_openai.so
```

`buildNativeCore` 已被钉在 `mergeReleaseJniLibFolders` 前面，会自动
触发 [build-android.sh](vendors/chat-openai/core/scripts/build-android.sh)
重编 .so。纯 Kotlin 改动会跳过 Rust 阶段。

### iOS XCFramework + Swift Package（全二进制发布）

```bash
cd app/sdks
./vendors/chat-openai/core/scripts/build-ios.sh \
  --out vendors/chat-openai/ios/LLMHubChatKit --profile release
# 产物: vendors/chat-openai/ios/LLMHubChatKit/LLMHubChatKit.xcframework
#   ├── ios-arm64/LLMHubChatKit.framework/
#   │   ├── LLMHubChatKit                       ← Rust .a + Swift .a 合并后的单一静态归档
#   │   └── Modules/LLMHubChatKit.swiftmodule/  ← 二进制 swiftmodule + .swiftinterface
#   └── ios-arm64_x86_64-simulator/LLMHubChatKit.framework/...
```

构建流水线：cargo build × 3 个 iOS triple → cbindgen 生成私有 C 头 →
swiftc 编译 facade（per-triple，开 `-enable-library-evolution`）→
`libtool -static` 把 Swift `.a` 和 Rust `.a` 合并成一个 framework
binary → `xcodebuild -create-xcframework` 打成最终 xcframework。

宿主 App 用 SPM 引入：

```swift
.package(path: "../path/to/app/sdks/vendors/chat-openai/ios/LLMHubChatKit")
// 或：.package(url: "https://github.com/.../LLMHubChatKit.git", from: "0.1.0")
```

注意：`Package.swift` 用 `.binaryTarget(path: "LLMHubChatKit.xcframework")`。
没有先跑 `build-ios.sh` 时 SPM 会因为路径缺失而拒绝解析。

## 在宿主 App 里使用

### Android (Kotlin)

```kotlin
val hub = LLMHubChat.create(
    baseUrl = "https://api.llmhub.io",
    apiKey  = BuildConfig.LLMHUB_API_KEY,
)
val resp = withContext(Dispatchers.IO) {
    hub.completions(skuId = "chat.gpt4o-class.standard").create(
        ChatCompletionRequest(messages = listOf(ChatMessage.user("hi")))
    )
}

// 流式
hub.completions(skuId = "chat.gpt4o-class.standard")
    .createStream(ChatCompletionRequest(messages = listOf(ChatMessage.user("hi"))))
    .collect { chunk ->
        chunk.choices.firstOrNull()?.delta?.content?.let(::print)
    }
```

### iOS (Swift)

```swift
import LLMHubChatKit                                       // 模块名带 Kit 后缀
                                                           // 类名仍是 LLMHubChat
let hub  = try LLMHubChat(baseURL: "https://api.llmhub.io", apiKey: "llmh-…")
let chat = try hub.completions(skuId: "chat.gpt4o-class.standard")

// 非流式
let r = try chat.create(.init(messages: [.user("hi")]))
print(r.choices.first?.message.content ?? "")

// 流式
for try await chunk in chat.createStream(.init(messages: [.user("hi")])) {
    if let s = chunk.choices.first?.delta.content { print(s, terminator: "") }
}
```

平台对应的 wire 合约见
[server.go](../services/internal/sdkapi/server.go) /
[issue.go](../services/internal/sdkapi/issue.go) /
[report.go](../services/internal/sdkapi/report.go)。
