package io.llmhub.chat

/**
 * Thrown by every [LLMHubChat] call. The native layer encodes its
 * stable error code in [code]; the human-facing reason lives in
 * [message]. Map `code` onto your app's UI strings — not `message`.
 *
 * Codes mirror `llmhub-core::Error::code` and are wire-stable.
 */
class LLMHubException(
    val code: String,
    message: String,
) : RuntimeException(message) {

    companion object {
        /** Parses a "code: message" payload as thrown by the JNI layer. */
        internal fun fromNative(raw: String): LLMHubException {
            val idx = raw.indexOf(": ")
            return if (idx > 0) LLMHubException(raw.substring(0, idx), raw.substring(idx + 2))
            else LLMHubException("unknown", raw)
        }
    }
}
