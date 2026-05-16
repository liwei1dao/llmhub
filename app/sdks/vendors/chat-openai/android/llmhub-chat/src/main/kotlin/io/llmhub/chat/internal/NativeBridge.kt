package io.llmhub.chat.internal

import io.llmhub.chat.StreamListener

/**
 * Direct JNI bindings into `libllmhub_chat_openai.so`. Never exposed
 * to consumers — `LLMHubChat` is the public facade. Everything here is
 * package-private to the SDK module.
 *
 * Method names + signatures must match the `Java_io_llmhub_chat_internal_NativeBridge_*`
 * symbols in the Rust FFI crate exactly. Any rename here is a wire
 * break — guarded by the consumer-rules.pro `-keep` rule.
 */
internal object NativeBridge {

    init {
        // System.loadLibrary("llmhub_chat_openai") → libllmhub_chat_openai.so
        System.loadLibrary("llmhub_chat_openai")
    }

    @JvmStatic external fun nativeNewPlatform(baseUrl: String, apiKey: String): Long
    @JvmStatic external fun nativeFreePlatform(handle: Long)

    @JvmStatic external fun nativeNewChat(platformHandle: Long, skuId: String): Long
    @JvmStatic external fun nativeFreeChat(handle: Long)

    @JvmStatic external fun nativeCreate(chatHandle: Long, requestJson: String): String
    @JvmStatic external fun nativeCreateStream(
        chatHandle: Long,
        requestJson: String,
        listener: StreamListener,
    )

    @JvmStatic external fun nativeListServices(platformHandle: Long): String
}
