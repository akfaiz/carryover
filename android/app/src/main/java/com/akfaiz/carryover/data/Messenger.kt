package com.akfaiz.carryover.data

import android.content.Context
import androidx.annotation.PluralsRes
import androidx.annotation.StringRes
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.CoreMessage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.receiveAsFlow

/** The app's texts, in its current language (the catalog in res/values and res/values-in). */
interface Texts {
    fun string(@StringRes id: Int, vararg args: Any): String
    fun plural(@PluralsRes id: Int, count: Int, vararg args: Any): String
}

/** [Texts] from the application's resources. */
class AndroidTexts(private val context: Context) : Texts {
    override fun string(id: Int, vararg args: Any): String = context.getString(id, *args)
    override fun plural(id: Int, count: Int, vararg args: Any): String = context.resources.getQuantityString(id, count, *args)
}

/**
 * What the view models tell the user: one-off messages for the snackbar, in
 * the app's language, shared by every view model (the shell shows them).
 */
class Messenger(private val texts: Texts) {
    private val channel = Channel<String>(Channel.BUFFERED)

    /** One-off messages for a snackbar. */
    val messages: Flow<String> = channel.receiveAsFlow()

    fun say(msg: String) {
        channel.trySend(msg)
    }

    /** A text of the app's catalog, in its current language. */
    fun str(@StringRes id: Int, vararg args: Any): String = texts.string(id, *args)

    fun plural(@PluralsRes id: Int, n: Int, vararg args: Any): String = texts.plural(id, n, *args)

    /** Says that [what] failed with [e] (cancellation is not a failure: it is thrown on). */
    fun failed(what: String, e: Exception) {
        if (e is CancellationException) throw e
        say(str(R.string.msg_error, what, error(e)))
    }

    /** What went wrong, in the app's language where the core said it with a code. */
    fun error(e: Throwable): String = texts.error(e)

    /** A field of the core ([english] with its [message]) in the app's language. */
    fun core(english: String, message: CoreMessage?): String = texts.core(english, message)

    /** Runs [block]; if it fails, says that [what] failed and returns null. */
    suspend fun <T> attempt(what: String, block: suspend () -> T): T? = try {
        block()
    } catch (e: Exception) {
        failed(what, e)
        null
    }
}
