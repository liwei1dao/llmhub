import Foundation
@_implementationOnly import LLMHubChatC

// =====================================================================
// Errors
// =====================================================================

/// Errors thrown by `LLMHubChat`. `code` mirrors `llmhub-core::Error::code`
/// — map your UI strings off `code`, not `message`.
public struct LLMHubError: Error, Sendable {
    public let code: String
    public let message: String
}

// =====================================================================
// Roles + content (text + multimodal parts)
// =====================================================================

/// Roles in the OpenAI chat protocol. Wire-stable.
public enum ChatRole: String, Codable, Sendable {
    case system, user, assistant, tool
}

/// Polymorphic message content. The OpenAI wire accepts either a
/// plain string (the common case) or an array of typed parts
/// (multimodal). Both round-trip through `Codable` here.
public enum MessageContent: Codable, Sendable {
    case text(String)
    case parts([ContentPart])

    public init(from decoder: any Decoder) throws {
        let c = try decoder.singleValueContainer()
        if let s = try? c.decode(String.self) { self = .text(s); return }
        if let a = try? c.decode([ContentPart].self) { self = .parts(a); return }
        throw DecodingError.dataCorruptedError(
            in: c,
            debugDescription: "content must be a JSON string or an array of parts"
        )
    }

    public func encode(to encoder: any Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .text(let s):  try c.encode(s)
        case .parts(let a): try c.encode(a)
        }
    }
}

/// One element inside a multimodal `content` array. JSON shape:
/// `{"type":"text","text":"…"}`, `{"type":"image_url","image_url":{…}}`,
/// `{"type":"input_audio","input_audio":{…}}`.
public enum ContentPart: Codable, Sendable {
    case text(String)
    case imageURL(ImageURL)
    case inputAudio(InputAudio)

    /// Explicit keys — the wire names are fixed by the OpenAI spec, so
    /// we don't rely on the encoder's snake_case strategy here.
    private enum CodingKeys: String, CodingKey {
        case type
        case text
        case imageURL   = "image_url"
        case inputAudio = "input_audio"
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let kind = try c.decode(String.self, forKey: .type)
        switch kind {
        case "text":
            self = .text(try c.decode(String.self, forKey: .text))
        case "image_url":
            self = .imageURL(try c.decode(ImageURL.self, forKey: .imageURL))
        case "input_audio":
            self = .inputAudio(try c.decode(InputAudio.self, forKey: .inputAudio))
        default:
            throw DecodingError.dataCorruptedError(
                forKey: .type, in: c,
                debugDescription: "unknown content part type: \(kind)"
            )
        }
    }

    public func encode(to encoder: any Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        switch self {
        case .text(let s):
            try c.encode("text", forKey: .type)
            try c.encode(s, forKey: .text)
        case .imageURL(let v):
            try c.encode("image_url", forKey: .type)
            try c.encode(v, forKey: .imageURL)
        case .inputAudio(let v):
            try c.encode("input_audio", forKey: .type)
            try c.encode(v, forKey: .inputAudio)
        }
    }
}

public struct ImageURL: Codable, Sendable {
    /// Either an `https://…` URL or `data:image/<mime>;base64,…`.
    public var url: String
    /// `"auto" | "low" | "high"`. Optional vision-tile budget hint.
    public var detail: String?

    public init(url: String, detail: String? = nil) {
        self.url = url; self.detail = detail
    }
}

public struct InputAudio: Codable, Sendable {
    /// Base64-encoded audio bytes.
    public var data: String
    /// `"wav" | "mp3"`.
    public var format: String

    public init(data: String, format: String) {
        self.data = data; self.format = format
    }
}

// =====================================================================
// JSONValue (raw JSON for tool schemas, response_format, etc.)
// =====================================================================

/// A raw JSON value used for fields the SDK forwards opaquely
/// (`FunctionDef.parameters`, `ChatCompletionRequest.responseFormat`,
/// custom tool_choice shapes). Keep keys in snake_case when constructing
/// `.object(…)` so they match the upstream's expected wire shape.
public enum JSONValue: Codable, Sendable, Hashable {
    case null
    case bool(Bool)
    case int(Int64)
    case double(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    public init(from decoder: any Decoder) throws {
        let c = try decoder.singleValueContainer()
        if c.decodeNil() { self = .null; return }
        if let v = try? c.decode(Bool.self)              { self = .bool(v);   return }
        if let v = try? c.decode(Int64.self)             { self = .int(v);    return }
        if let v = try? c.decode(Double.self)            { self = .double(v); return }
        if let v = try? c.decode(String.self)            { self = .string(v); return }
        if let v = try? c.decode([JSONValue].self)       { self = .array(v);  return }
        if let v = try? c.decode([String: JSONValue].self) { self = .object(v); return }
        throw DecodingError.dataCorruptedError(in: c, debugDescription: "unrecognised JSON value")
    }

    public func encode(to encoder: any Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .null:          try c.encodeNil()
        case .bool(let v):   try c.encode(v)
        case .int(let v):    try c.encode(v)
        case .double(let v): try c.encode(v)
        case .string(let v): try c.encode(v)
        case .array(let v):  try c.encode(v)
        case .object(let v): try c.encode(v)
        }
    }
}

// =====================================================================
// Tools (function calling)
// =====================================================================

/// One tool the model is allowed to call. Today only `kind="function"`
/// exists in the OpenAI wire; the shape is kept so future kinds drop in.
public struct Tool: Codable, Sendable {
    public var kind: String        // -> "type" on the wire
    public var function: FunctionDef

    public init(function: FunctionDef, kind: String = "function") {
        self.kind = kind; self.function = function
    }

    private enum CodingKeys: String, CodingKey { case kind = "type", function }
}

public struct FunctionDef: Codable, Sendable {
    public var name: String
    public var description: String?
    /// Raw JSON Schema describing the function's parameters.
    public var parameters: JSONValue

    public init(name: String, description: String? = nil, parameters: JSONValue) {
        self.name = name; self.description = description; self.parameters = parameters
    }
}

/// `tool_choice` — `"auto"`, `"none"`, `"required"`, or a specific
/// function. Encoded as the wire union shape OpenAI accepts.
public enum ToolChoice: Codable, Sendable {
    case auto
    case none
    case required
    case function(name: String)

    public init(from decoder: any Decoder) throws {
        let c = try decoder.singleValueContainer()
        if let s = try? c.decode(String.self) {
            switch s {
            case "auto":     self = .auto;     return
            case "none":     self = .none;     return
            case "required": self = .required; return
            default: break
            }
        }
        struct Specific: Decodable { let type: String; let function: FuncName
            struct FuncName: Decodable { let name: String } }
        let s = try c.decode(Specific.self)
        guard s.type == "function" else {
            throw DecodingError.dataCorruptedError(in: c, debugDescription: "unknown tool_choice type: \(s.type)")
        }
        self = .function(name: s.function.name)
    }

    public func encode(to encoder: any Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .auto:     try c.encode("auto")
        case .none:     try c.encode("none")
        case .required: try c.encode("required")
        case .function(let name):
            struct Specific: Encodable { let type: String; let function: FuncName
                struct FuncName: Encodable { let name: String } }
            try c.encode(Specific(type: "function", function: .init(name: name)))
        }
    }
}

/// Assistant-emitted tool call. `arguments` is JSON **encoded as a
/// string** — that's the upstream's wire convention; do not double-encode.
public struct ToolCall: Codable, Sendable {
    public var id: String
    public var kind: String        // -> "type" on the wire
    public var function: ToolFunctionCall

    public init(id: String, function: ToolFunctionCall, kind: String = "function") {
        self.id = id; self.kind = kind; self.function = function
    }

    private enum CodingKeys: String, CodingKey { case id, kind = "type", function }
}

public struct ToolFunctionCall: Codable, Sendable {
    public var name: String
    public var arguments: String

    public init(name: String, arguments: String) {
        self.name = name; self.arguments = arguments
    }
}

// =====================================================================
// Messages
// =====================================================================

public struct ChatMessage: Codable, Sendable {
    public var role: ChatRole
    public var content: MessageContent
    public var name: String?
    /// Assistant-side: function calls the model wants the host to execute.
    public var toolCalls: [ToolCall]?
    /// Tool-result-side: id of the assistant's tool_call this answers.
    public var toolCallId: String?

    public init(
        role: ChatRole,
        content: MessageContent,
        name: String? = nil,
        toolCalls: [ToolCall]? = nil,
        toolCallId: String? = nil
    ) {
        self.role = role; self.content = content; self.name = name
        self.toolCalls = toolCalls; self.toolCallId = toolCallId
    }

    // Convenience: plain-text messages
    public static func user(_ s: String)      -> ChatMessage { .init(role: .user,      content: .text(s)) }
    public static func system(_ s: String)    -> ChatMessage { .init(role: .system,    content: .text(s)) }
    public static func assistant(_ s: String) -> ChatMessage { .init(role: .assistant, content: .text(s)) }

    // Convenience: multimodal user message
    public static func user(_ parts: ContentPart...) -> ChatMessage {
        .init(role: .user, content: .parts(parts))
    }

    // Convenience: assistant emits tool calls
    public static func assistant(toolCalls: [ToolCall]) -> ChatMessage {
        .init(role: .assistant, content: .text(""), toolCalls: toolCalls)
    }

    // Convenience: tool-result message
    public static func tool(callId: String, result: String) -> ChatMessage {
        .init(role: .tool, content: .text(result), toolCallId: callId)
    }
}

// =====================================================================
// Request / response envelopes
// =====================================================================

public struct ChatCompletionRequest: Codable, Sendable {
    public var model: String
    public var messages: [ChatMessage]
    public var temperature: Float?
    public var topP: Float?
    public var maxTokens: Int?
    public var stream: Bool
    public var stop: [String]?

    /// Function tools the model may call.
    public var tools: [Tool]?
    /// `"auto"` / `"none"` / `"required"` / a specific function.
    public var toolChoice: ToolChoice?
    /// OpenAI's parallel-tool-call toggle.
    public var parallelToolCalls: Bool?
    /// Response-format hint, e.g. `{"type":"json_object"}`.
    public var responseFormat: JSONValue?

    public init(
        model: String = "",
        messages: [ChatMessage],
        temperature: Float? = nil,
        topP: Float? = nil,
        maxTokens: Int? = nil,
        stream: Bool = false,
        stop: [String]? = nil,
        tools: [Tool]? = nil,
        toolChoice: ToolChoice? = nil,
        parallelToolCalls: Bool? = nil,
        responseFormat: JSONValue? = nil
    ) {
        self.model = model; self.messages = messages
        self.temperature = temperature; self.topP = topP
        self.maxTokens = maxTokens; self.stream = stream; self.stop = stop
        self.tools = tools; self.toolChoice = toolChoice
        self.parallelToolCalls = parallelToolCalls
        self.responseFormat = responseFormat
    }
}

public struct Usage: Codable, Sendable {
    public var promptTokens: Int64 = 0
    public var completionTokens: Int64 = 0
    public var totalTokens: Int64 = 0
}

public struct ChatCompletionChoice: Codable, Sendable {
    public var index: Int = 0
    public var message: ChatMessage
    public var finishReason: String?
}

public struct ChatCompletion: Codable, Sendable {
    public var id: String = ""
    public var model: String = ""
    public var choices: [ChatCompletionChoice]
    public var usage: Usage = .init()
}

// =====================================================================
// Streaming
// =====================================================================

public struct ChatDelta: Codable, Sendable {
    public var role: String?
    public var content: String?
    public var toolCalls: [ToolCallDelta]?

    public init(role: String? = nil, content: String? = nil, toolCalls: [ToolCallDelta]? = nil) {
        self.role = role; self.content = content; self.toolCalls = toolCalls
    }
}

public struct ToolCallDelta: Codable, Sendable {
    public var index: Int
    public var id: String?
    public var kind: String?       // -> "type" on the wire
    public var function: ToolFunctionCallDelta?

    private enum CodingKeys: String, CodingKey { case index, id, kind = "type", function }
}

public struct ToolFunctionCallDelta: Codable, Sendable {
    public var name: String?
    public var arguments: String?
}

public struct ChatChunkChoice: Codable, Sendable {
    public var index: Int = 0
    public var delta: ChatDelta = .init()
    public var finishReason: String?
}

public struct ChatCompletionChunk: Codable, Sendable {
    public var id: String = ""
    public var model: String = ""
    public var choices: [ChatChunkChoice]
    public var usage: Usage?
}

// =====================================================================
// Service discovery
// =====================================================================

public struct ServiceEntry: Codable, Sendable {
    public var skuId: String
    public var displayName: String = ""
    public var categoryId: String = ""
    public var capability: String = ""
    public var quotaTotal: Int64 = 0
    public var quotaUsed: Int64 = 0
    public var quotaRemaining: Int64 = 0
    public var qpsLimit: Int32 = 0
    public var billingUnit: String = ""
    public var periodEnd: String = ""
    public var planKind: String = ""
    public var planName: String = ""
}

// MARK: - JSON envelope returned by the C layer
// `{"ok":true,"data":...}` or `{"ok":false,"error":{"code":..,"message":..}}`

private struct OkEnvelope<T: Decodable>: Decodable { let ok: Bool; let data: T }
private struct ErrEnvelope: Decodable {
    let ok: Bool
    let error: ErrBody
    struct ErrBody: Decodable { let code: String; let message: String }
}

private enum JSON {
    static let decoder: JSONDecoder = {
        let d = JSONDecoder()
        d.keyDecodingStrategy   = .convertFromSnakeCase
        return d
    }()
    static let encoder: JSONEncoder = {
        let e = JSONEncoder()
        e.keyEncodingStrategy   = .convertToSnakeCase
        return e
    }()
}

/// Public entry point. Mirrors `io.llmhub.chat.LLMHubChat` on Android.
///
/// ```swift
/// let hub = try LLMHubChat(baseURL: "https://api.llmhub.io", apiKey: "llmh-…")
/// let chat = try hub.completions(skuId: "chat.gpt4o-class.standard")
/// let r = try chat.create(.init(messages: [.user("hi")]))
/// print(r.choices.first?.message.content ?? "")
/// ```
public final class LLMHubChat: @unchecked Sendable {

    private let platformHandle: OpaquePointer

    public init(baseURL: String, apiKey: String) throws {
        guard !baseURL.isEmpty, !apiKey.isEmpty else {
            throw LLMHubError(code: "invalid_argument", message: "baseURL and apiKey must not be empty")
        }
        guard let h = llmhub_platform_new(baseURL, apiKey) else {
            throw LLMHubError(code: "init_failed", message: "native platform init failed")
        }
        self.platformHandle = OpaquePointer(h)
    }

    deinit {
        llmhub_platform_free(UnsafeMutableRawPointer(platformHandle))
    }

    public func completions(skuId: String) throws -> ChatSession {
        guard !skuId.isEmpty else {
            throw LLMHubError(code: "invalid_argument", message: "skuId must not be empty")
        }
        guard let h = llmhub_chat_new(UnsafeMutableRawPointer(platformHandle), skuId) else {
            throw LLMHubError(code: "init_failed", message: "native chat init failed")
        }
        return ChatSession(handle: OpaquePointer(h))
    }

    public func listServices() throws -> [ServiceEntry] {
        let raw = llmhub_platform_list_services(UnsafeMutableRawPointer(platformHandle))
        return try Self.decode(raw)
    }

    fileprivate static func decode<T: Decodable>(_ raw: UnsafeMutablePointer<CChar>?) throws -> T {
        guard let raw else {
            throw LLMHubError(code: "native_error", message: "null response")
        }
        defer { llmhub_string_free(raw) }
        let data = Data(bytes: raw, count: strlen(raw))
        if let ok = try? JSON.decoder.decode(OkEnvelope<T>.self, from: data) {
            return ok.data
        }
        if let err = try? JSON.decoder.decode(ErrEnvelope.self, from: data) {
            throw LLMHubError(code: err.error.code, message: err.error.message)
        }
        throw LLMHubError(code: "decode_error", message: "unrecognised native envelope")
    }
}

// MARK: - Chat session

extension LLMHubChat {
    public final class ChatSession: @unchecked Sendable {
        private let handle: OpaquePointer

        fileprivate init(handle: OpaquePointer) { self.handle = handle }

        deinit { llmhub_chat_free(UnsafeMutableRawPointer(handle)) }

        /// One-shot completion. Blocking on the calling thread — invoke
        /// from `Task.detached { ... }` or a background queue.
        public func create(_ request: ChatCompletionRequest) throws -> ChatCompletion {
            let body = try JSON.encoder.encode(request)
            guard let bodyStr = String(data: body, encoding: .utf8) else {
                throw LLMHubError(code: "encode_error", message: "could not utf8-encode request")
            }
            let raw = llmhub_chat_create(UnsafeMutableRawPointer(handle), bodyStr)
            return try LLMHubChat.decode(raw)
        }

        /// Streaming completion as an `AsyncThrowingStream`. Cancelling
        /// the task cancels the underlying SSE — the native side honours
        /// the chunk-callback returning `false`.
        public func createStream(
            _ request: ChatCompletionRequest
        ) -> AsyncThrowingStream<ChatCompletionChunk, Error> {
            var req = request; req.stream = true
            let body: String
            do {
                let data = try JSON.encoder.encode(req)
                guard let s = String(data: data, encoding: .utf8) else {
                    return AsyncThrowingStream { c in
                        c.finish(throwing: LLMHubError(code: "encode_error", message: "utf8"))
                    }
                }
                body = s
            } catch {
                return AsyncThrowingStream { c in c.finish(throwing: error) }
            }

            let handle = self.handle
            return AsyncThrowingStream { continuation in
                let task = Task.detached(priority: .userInitiated) {
                    let ctx = StreamContext(continuation: continuation)
                    let ctxPtr = Unmanaged.passRetained(ctx).toOpaque()
                    defer { Unmanaged<StreamContext>.fromOpaque(ctxPtr).release() }

                    llmhub_chat_create_stream(
                        UnsafeMutableRawPointer(handle),
                        body,
                        ctxPtr,
                        { user, json in
                            guard let user, let json else { return false }
                            let ctx = Unmanaged<StreamContext>.fromOpaque(user).takeUnretainedValue()
                            return ctx.onChunk(cString: json)
                        },
                        { user in
                            guard let user else { return }
                            let ctx = Unmanaged<StreamContext>.fromOpaque(user).takeUnretainedValue()
                            ctx.continuation.finish()
                        },
                        { user, code, message in
                            guard let user, let code, let message else { return }
                            let ctx = Unmanaged<StreamContext>.fromOpaque(user).takeUnretainedValue()
                            ctx.continuation.finish(throwing: LLMHubError(
                                code: String(cString: code),
                                message: String(cString: message)
                            ))
                        }
                    )
                }
                continuation.onTermination = { _ in task.cancel() }
            }
        }
    }
}

// MARK: - Stream callback context

private final class StreamContext: @unchecked Sendable {
    let continuation: AsyncThrowingStream<ChatCompletionChunk, Error>.Continuation
    init(continuation: AsyncThrowingStream<ChatCompletionChunk, Error>.Continuation) {
        self.continuation = continuation
    }

    /// Returns `false` to ask the native layer to stop the stream.
    func onChunk(cString: UnsafePointer<CChar>) -> Bool {
        let data = Data(bytes: cString, count: strlen(cString))
        guard let chunk = try? JSON.decoder.decode(ChatCompletionChunk.self, from: data) else {
            continuation.finish(throwing: LLMHubError(code: "decode_error", message: "chunk"))
            return false
        }
        switch continuation.yield(chunk) {
        case .terminated:                 return false
        case .dropped, .enqueued:         return true
        @unknown default:                 return true
        }
    }
}
