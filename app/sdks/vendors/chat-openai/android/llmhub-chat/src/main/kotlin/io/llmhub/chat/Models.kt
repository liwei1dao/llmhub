package io.llmhub.chat

import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonDecoder
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive

// =====================================================================
// Roles + content (text + multimodal parts)
// =====================================================================

@Serializable
enum class ChatRole {
    @SerialName("system")    SYSTEM,
    @SerialName("user")      USER,
    @SerialName("assistant") ASSISTANT,
    @SerialName("tool")      TOOL,
}

/**
 * Polymorphic message content. The OpenAI wire accepts either a plain
 * string (the common case) or an array of typed parts (multimodal). We
 * model both via a sealed class with a custom serializer so the JSON
 * round-trips unchanged either way.
 */
@Serializable(with = MessageContentSerializer::class)
sealed class MessageContent {
    /** Text-only message body. */
    data class Text(val text: String) : MessageContent()

    /** Multimodal message body — an ordered list of [ContentPart]s. */
    data class Parts(val parts: List<ContentPart>) : MessageContent()

    companion object {
        fun text(s: String): MessageContent = Text(s)
        fun parts(vararg p: ContentPart): MessageContent = Parts(p.toList())
        fun parts(p: List<ContentPart>): MessageContent = Parts(p)
    }
}

/** One element inside a multimodal `content` array. */
@OptIn(ExperimentalSerializationApi::class)
@Serializable
sealed class ContentPart {
    @Serializable @SerialName("text")
    data class TextPart(val text: String) : ContentPart()

    @Serializable @SerialName("image_url")
    data class Image(val imageUrl: ImageUrl) : ContentPart()

    @Serializable @SerialName("input_audio")
    data class Audio(val inputAudio: InputAudio) : ContentPart()
}

@Serializable
data class ImageUrl(
    /** Either an `https://…` URL or `data:image/<mime>;base64,…`. */
    val url: String,
    /** `"auto" | "low" | "high"` — optional vision-tile budget hint. */
    val detail: String? = null,
)

@Serializable
data class InputAudio(
    /** Base64-encoded audio bytes. */
    val data: String,
    /** `"wav" | "mp3"`. */
    val format: String,
)

/** Custom serializer: emits a JSON string for [MessageContent.Text],
 *  a JSON array for [MessageContent.Parts]. Symmetric on decode. */
internal object MessageContentSerializer : KSerializer<MessageContent> {

    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("MessageContent", PrimitiveKind.STRING)

    private val partsSerializer = ListSerializer(ContentPart.serializer())

    override fun serialize(encoder: Encoder, value: MessageContent) {
        when (value) {
            is MessageContent.Text  -> encoder.encodeString(value.text)
            is MessageContent.Parts -> encoder.encodeSerializableValue(partsSerializer, value.parts)
        }
    }

    override fun deserialize(decoder: Decoder): MessageContent {
        require(decoder is JsonDecoder) {
            "MessageContent must be (de)serialised via the JSON format."
        }
        return when (val el: JsonElement = decoder.decodeJsonElement()) {
            is JsonPrimitive -> {
                if (!el.isString) {
                    throw SerializationException("expected JSON string for content, got $el")
                }
                MessageContent.Text(el.content)
            }
            is JsonArray -> MessageContent.Parts(
                decoder.json.decodeFromJsonElement(partsSerializer, el)
            )
            else -> throw SerializationException(
                "expected JSON string or array for content, got $el"
            )
        }
    }
}

// =====================================================================
// Tools (function calling)
// =====================================================================

@Serializable
data class Tool(
    /** Always `"function"` today. Kept as a field so future kinds drop in. */
    @SerialName("type") val kind: String = "function",
    val function: FunctionDef,
)

@Serializable
data class FunctionDef(
    val name: String,
    val description: String? = null,
    /** Raw JSON Schema describing the function parameters. */
    val parameters: JsonElement,
)

/** Assistant-emitted call. `arguments` is JSON encoded **as a string** —
 *  that is the upstream's wire convention. Do not double-encode it. */
@Serializable
data class ToolCall(
    val id: String,
    @SerialName("type") val kind: String = "function",
    val function: ToolFunctionCall,
)

@Serializable
data class ToolFunctionCall(
    val name: String,
    val arguments: String,
)

// =====================================================================
// Messages
// =====================================================================

@Serializable
data class ChatMessage(
    val role: ChatRole,
    val content: MessageContent,
    val name: String? = null,
    /** Assistant-side: function calls the model wants the host to execute. */
    val toolCalls: List<ToolCall>? = null,
    /** Tool-result-side: id from the assistant's [toolCalls] this answers. */
    val toolCallId: String? = null,
) {
    companion object {
        /** Convenience: plain-text user / system / assistant messages. */
        fun user(text: String)      = ChatMessage(ChatRole.USER,      MessageContent.Text(text))
        fun system(text: String)    = ChatMessage(ChatRole.SYSTEM,    MessageContent.Text(text))
        fun assistant(text: String) = ChatMessage(ChatRole.ASSISTANT, MessageContent.Text(text))

        /** Convenience: multimodal user message. */
        fun user(vararg parts: ContentPart) =
            ChatMessage(ChatRole.USER, MessageContent.Parts(parts.toList()))

        /** Convenience: assistant message that emits tool calls. */
        fun assistant(toolCalls: List<ToolCall>) =
            ChatMessage(ChatRole.ASSISTANT, MessageContent.Text(""), toolCalls = toolCalls)

        /** Convenience: tool-result message bound to a specific call id. */
        fun tool(toolCallId: String, result: String) =
            ChatMessage(ChatRole.TOOL, MessageContent.Text(result), toolCallId = toolCallId)
    }
}

// =====================================================================
// Request / response envelopes
// =====================================================================

/**
 * Mirrors the OpenAI `chat/completions` request body. `model` is
 * intentionally optional: when blank the SDK populates it from the
 * lease's `upstream_model`, so consumers pass `skuId` and don't need
 * to know what vendor sits behind it.
 */
@Serializable
data class ChatCompletionRequest(
    val model: String = "",
    val messages: List<ChatMessage>,
    val temperature: Float? = null,
    val topP: Float? = null,
    val maxTokens: Int? = null,
    val stream: Boolean = false,
    val stop: List<String>? = null,
    /** Function tools the model may call. */
    val tools: List<Tool>? = null,
    /** `"auto" | "none" | "required"` or
     *  `{"type":"function","function":{"name":"my_fn"}}`. Forwarded raw —
     *  different upstreams accept slightly different shapes. */
    val toolChoice: JsonElement? = null,
    /** OpenAI's parallel-tool-call toggle; default `true` upstream. */
    val parallelToolCalls: Boolean? = null,
    /** Response-format hint, e.g. `{"type":"json_object"}` for JSON
     *  mode or `{"type":"json_schema", …}` for structured output. */
    val responseFormat: JsonElement? = null,
)

@Serializable
data class Usage(
    val promptTokens: Long = 0,
    val completionTokens: Long = 0,
    val totalTokens: Long = 0,
)

@Serializable
data class ChatCompletionChoice(
    val index: Int = 0,
    val message: ChatMessage,
    val finishReason: String? = null,
)

@Serializable
data class ChatCompletion(
    val id: String = "",
    val model: String = "",
    val choices: List<ChatCompletionChoice>,
    val usage: Usage = Usage(),
)

// =====================================================================
// Streaming
// =====================================================================

/**
 * Streaming delta. `content` accumulates text chunks; `toolCalls`
 * carries incremental updates to each tool call by `index`. The
 * consumer is responsible for re-stitching by index across chunks —
 * the SDK exposes the raw frames so it stays protocol-thin.
 */
@Serializable
data class ChatDelta(
    val role: String? = null,
    val content: String? = null,
    val toolCalls: List<ToolCallDelta>? = null,
)

@Serializable
data class ToolCallDelta(
    val index: Int,
    val id: String? = null,
    @SerialName("type") val kind: String? = null,
    val function: ToolFunctionCallDelta? = null,
)

@Serializable
data class ToolFunctionCallDelta(
    val name: String? = null,
    val arguments: String? = null,
)

@Serializable
data class ChatChunkChoice(
    val index: Int = 0,
    val delta: ChatDelta = ChatDelta(),
    val finishReason: String? = null,
)

@Serializable
data class ChatCompletionChunk(
    val id: String = "",
    val model: String = "",
    val choices: List<ChatChunkChoice>,
    val usage: Usage? = null,
)

// =====================================================================
// Service discovery
// =====================================================================

@Serializable
data class ServiceEntry(
    val skuId: String,
    val planKind: String = "",
    val planName: String = "",
    val displayName: String = "",
    val categoryId: String = "",
    val billingUnit: String = "",
    val capability: String = "",
    val quotaTotal: Long = 0,
    val quotaUsed: Long = 0,
    val quotaRemaining: Long = 0,
    val qpsLimit: Int = 0,
    val periodEnd: String = "",
)
