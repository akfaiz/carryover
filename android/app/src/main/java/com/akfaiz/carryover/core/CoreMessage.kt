package com.akfaiz.carryover.core

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

/**
 * A text of the Go core that the app can translate (internal/app/msg): its code
 * with the values of its placeholders, what it is about ([cause], shown after it
 * and ": "), and the whole [text] in English. Translate one with
 * `Texts.core` (data/CoreText.kt).
 */
@Serializable
data class CoreMessage(
    /** "" for text without a code, such as an error of the system or rclone: it stays as it is. */
    val code: String = "",
    /** Strings, numbers, and lists of messages. */
    val args: Map<String, JsonElement> = emptyMap(),
    val cause: CoreMessage? = null,
    val text: String = "",
) {
    /** It, or its cause, has a code. */
    val translatable: Boolean get() = code.isNotEmpty() || cause?.translatable == true
}

/** An error of the Go core with a text the app can translate; [message] is the English. */
class CoreException(val detail: CoreMessage, cause: Throwable? = null) : Exception(detail.text, cause)

/**
 * [e] as a [CoreException] when its text carries a message: [prefix] followed by
 * the message as JSON (mobile.ErrorPrefix, as only an error's text reaches
 * Kotlin). Any other error is returned as it is.
 */
fun coreError(e: Exception, prefix: String): Exception {
    val payload = e.message?.takeIf { it.startsWith(prefix) }?.removePrefix(prefix) ?: return e
    val detail = runCatching { json.decodeFromString<CoreMessage>(payload) }.getOrNull() ?: return e
    return CoreException(detail, e)
}
