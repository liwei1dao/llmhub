package io.llmhub.chat

/**
 * Receives the per-chunk stream callbacks from the native layer. JNI
 * looks up the methods by exact name + signature — DO NOT rename or
 * remove parameters without updating the Rust side in lockstep.
 *
 * Threading: every callback fires on whichever thread invoked
 * `LLMHubChat.createStream(...)`. If you started the call from
 * `Dispatchers.IO`, callbacks arrive on `Dispatchers.IO`. UI updates
 * must hop to the main thread themselves.
 */
interface StreamListener {
    /**
     * Called once per SSE chunk. The JSON is the raw chunk envelope
     * (OpenAI `chat.completion.chunk` shape). Return `false` to
     * cancel the stream — `onComplete` will still fire so cleanup
     * code can run in one place.
     */
    fun onChunk(json: String): Boolean

    /** Stream ended normally — either upstream sent [DONE] or you cancelled. */
    fun onComplete()

    /**
     * Stream failed. `code` is the stable identifier (see
     * [LLMHubException.code]); `message` is the human reason.
     */
    fun onError(code: String, message: String)
}
