package io.llmhub.chat

import io.llmhub.chat.internal.NativeBridge
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNamingStrategy
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Entry point for the chat capability. Construct one per LLMHub
 * account (it owns the per-process lease cache that lives inside the
 * native library) and reuse it across all sku-based chat sessions.
 *
 * ```
 * val hub = LLMHubChat.create(
 *     baseUrl = "https://api.llmhub.io",
 *     apiKey  = "llmh-xxxx",
 * )
 *
 * val resp = hub.completions("gpt4o-class").create(
 *     ChatCompletionRequest(messages = listOf(ChatMessage.user("hi")))
 * )
 * ```
 */
class LLMHubChat private constructor(
    private val platformHandle: Long,
) : AutoCloseable {

    private val closed = AtomicBoolean(false)

    /** Returns a chat session bound to `skuId`. Cheap; do not cache. */
    fun completions(skuId: String): ChatSession {
        require(!closed.get()) { "LLMHubChat has been closed" }
        require(skuId.isNotBlank()) { "skuId must not be blank" }
        val chatHandle = NativeBridge.nativeNewChat(platformHandle, skuId)
        if (chatHandle == 0L) {
            throw LLMHubException("init_failed", "native chat init failed")
        }
        return ChatSession(chatHandle)
    }

    /** Returns the subscribed SKUs for the bound api_key. */
    fun listServices(): List<ServiceEntry> {
        require(!closed.get()) { "LLMHubChat has been closed" }
        val json = NativeBridge.nativeListServices(platformHandle)
        return JSON.decodeFromString(json)
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            NativeBridge.nativeFreePlatform(platformHandle)
        }
    }

    /** A handle to a sku-bound chat session. Reuse across many calls. */
    inner class ChatSession internal constructor(
        private val chatHandle: Long,
    ) : AutoCloseable {

        private val sessionClosed = AtomicBoolean(false)

        /** One-shot completion. Blocking — call from IO dispatcher. */
        fun create(request: ChatCompletionRequest): ChatCompletion {
            check(!sessionClosed.get()) { "ChatSession has been closed" }
            val payload = JSON.encodeToString(ChatCompletionRequest.serializer(), request)
            val raw = NativeBridge.nativeCreate(chatHandle, payload)
            return JSON.decodeFromString(ChatCompletion.serializer(), raw)
        }

        /**
         * Streaming completion as a cold [Flow] of chunks. Cancelling
         * the flow's collector cancels the underlying SSE stream and
         * triggers a best-effort usage report.
         */
        fun createStream(request: ChatCompletionRequest): Flow<ChatCompletionChunk> = callbackFlow {
            check(!sessionClosed.get()) { "ChatSession has been closed" }
            val streaming = request.copy(stream = true)
            val payload = JSON.encodeToString(ChatCompletionRequest.serializer(), streaming)

            val listener = object : StreamListener {
                override fun onChunk(json: String): Boolean {
                    val chunk = try {
                        JSON.decodeFromString(ChatCompletionChunk.serializer(), json)
                    } catch (e: Throwable) {
                        close(LLMHubException("decode_error", e.message ?: "chunk decode failed"))
                        return false
                    }
                    val sent = trySend(chunk).isSuccess
                    return sent && !isClosedForSend
                }
                override fun onComplete() { close() }
                override fun onError(code: String, message: String) {
                    close(LLMHubException(code, message))
                }
            }

            // The native call blocks the caller's thread until the
            // stream ends. We park the bridge on a single-thread executor
            // via flowOn() below so the collector's coroutine is never
            // blocked.
            try {
                NativeBridge.nativeCreateStream(chatHandle, payload, listener)
            } catch (e: LLMHubException) {
                close(e)
            } catch (e: Throwable) {
                close(LLMHubException("native_error", e.message ?: "native call failed"))
            }
            awaitClose { /* nothing — cancellation is signalled via onChunk return */ }
        }.flowOn(Dispatchers.IO)

        override fun close() {
            if (sessionClosed.compareAndSet(false, true)) {
                NativeBridge.nativeFreeChat(chatHandle)
            }
        }
    }

    companion object {
        @PublishedApi
        @OptIn(ExperimentalSerializationApi::class)
        internal val JSON = Json {
            ignoreUnknownKeys = true
            encodeDefaults    = false
            explicitNulls     = false
            namingStrategy    = JsonNamingStrategy.SnakeCase
        }

        /**
         * Build a new client.
         *
         * @param baseUrl LLMHub deployment, e.g. `https://api.llmhub.io`.
         *                No trailing slash; the SDK appends the well-
         *                known paths internally.
         * @param apiKey  The user's LLMHub api_key (NOT an upstream
         *                provider key). The native layer never logs
         *                this and the upstream credential it resolves
         *                to never crosses JNI back into Kotlin.
         */
        @JvmStatic
        fun create(baseUrl: String, apiKey: String): LLMHubChat {
            require(baseUrl.isNotBlank()) { "baseUrl must not be blank" }
            require(apiKey.isNotBlank())  { "apiKey must not be blank" }
            val handle = NativeBridge.nativeNewPlatform(baseUrl, apiKey)
            if (handle == 0L) {
                throw LLMHubException("init_failed", "native platform init failed")
            }
            return LLMHubChat(handle)
        }
    }
}
