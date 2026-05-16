// swift-tools-version: 5.9
//
// LLMHubChatKit — binary-only Swift Package for the chat-openai SDK.
//
// The package ships exactly one artefact: `LLMHubChatKit.xcframework`.
// Both the Rust core (libllmhub_chat_openai.a) AND the Swift facade
// are pre-compiled into that XCFramework. Consumers see only the
// public API surface that Swift's library-evolution mode emits to
// `.swiftinterface` (function signatures, no implementation).
//
// Module name is `LLMHubChatKit` instead of `LLMHubChat` to avoid the
// "module name == top-level class name" collision in `.swiftinterface`
// qualified references. Consumers import the Kit module but still call
// the friendly `LLMHubChat` class:
//
//     import LLMHubChatKit
//     let hub = try LLMHubChat(baseURL: "...", apiKey: "...")
//
// Build with `vendors/chat-openai/core/scripts/build-ios.sh`. SPM
// refuses to resolve the package unless the .xcframework is on disk.
import PackageDescription

let package = Package(
    name: "LLMHubChatKit",
    platforms: [
        .iOS(.v15),
        .macOS(.v12),
    ],
    products: [
        .library(name: "LLMHubChatKit", targets: ["LLMHubChatKit"]),
    ],
    targets: [
        .binaryTarget(
            name: "LLMHubChatKit",
            path: "LLMHubChatKit.xcframework"
        ),
    ]
)
