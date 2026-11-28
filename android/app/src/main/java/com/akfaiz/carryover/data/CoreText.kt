package com.akfaiz.carryover.data

import androidx.annotation.PluralsRes
import androidx.annotation.StringRes
import com.akfaiz.carryover.core.CoreException
import com.akfaiz.carryover.core.CoreMessage
import com.akfaiz.carryover.core.json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement

/** The resource of one of the core's messages (see [CORE_TEXTS]); [args] name its placeholders in order. */
internal sealed class CoreString(val args: List<String>) {
    class Text(@param:StringRes val res: Int, vararg args: String) : CoreString(args.toList())
    class Plural(@param:PluralsRes val res: Int, vararg args: String) : CoreString(args.toList())
}

/** The placeholder that picks a plural's form (msg.CountArg). */
private const val COUNT = "Count"

/** [m] in the app's language. A code this app doesn't know (a newer core), and text without one, stay in English. */
fun Texts.core(m: CoreMessage): String {
    if (!m.translatable) return m.text
    if (m.code.isEmpty()) {
        // Untranslatable words in front of a translatable cause.
        val cause = m.cause ?: return m.text
        val suffix = ": ${cause.text}"
        return if (m.text.endsWith(suffix)) m.text.removeSuffix(suffix) + ": " + core(cause) else m.text
    }
    val entry = CORE_TEXTS[m.code] ?: return m.text
    val values: Array<Any> = entry.args.map { argument(m.args[it]) }.toTypedArray()
    val text = when (entry) {
        is CoreString.Text -> string(entry.res, *values)
        is CoreString.Plural -> plural(entry.res, (m.args[COUNT] as? JsonPrimitive)?.content?.toIntOrNull() ?: 0, *values)
    }
    return m.cause?.let { "$text: ${core(it)}" } ?: text
}

private fun Texts.argument(value: JsonElement?): String = when (value) {
    is JsonArray -> value.joinToString(", ") { core(json.decodeFromJsonElement<CoreMessage>(it)) }
    is JsonPrimitive -> value.content
    else -> ""
}

/** A field of the core in the app's language: [message] translated, or [english] when it has none. */
fun Texts.core(english: String, message: CoreMessage?): String = message?.let { core(it) } ?: english

/** What went wrong, in the app's language where the core said it with a code. */
fun Texts.error(e: Throwable): String = if (e is CoreException) core(e.detail) else e.message ?: e.toString()
