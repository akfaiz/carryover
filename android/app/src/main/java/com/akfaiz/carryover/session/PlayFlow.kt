package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.BeginOutcome
import com.akfaiz.carryover.core.NATIVE_PROFILE
import com.akfaiz.carryover.core.SessionCore
import kotlinx.coroutines.CancellationException

/** How starting a play session went. */
sealed interface PlayOutcome {
    /** Started; [offline]: with no network, so the game starts without syncing. */
    data class Started(val offline: Boolean) : PlayOutcome
    data object Cancelled : PlayOutcome
    data class Failed(val message: String) : PlayOutcome
}

/**
 * The rules of starting a play session (see [Sessions], which gives it Android
 * parts): wait for the upload of the previous session of the game, sync before
 * playing (questions arrive as prompt events), take the cloud lock, hand the
 * session to the watcher, then start the game. No Android classes, so it is
 * unit tested against a fake [SessionCore].
 */
class PlayFlow(
    private val core: SessionCore,
    private val manager: SessionManager,
    /** Hands the session of key, whose game runs in pkg, to the watcher. */
    private val watch: (key: String, pkg: String, title: String) -> Unit,
    /** Ends the session of key (the game didn't start). */
    private val stopWatching: (key: String) -> Unit,
    /** The message for a session that couldn't start. */
    private val startFailed: (title: String, cause: String) -> String,
) {
    /**
     * Plays profile [key] ([native]: the game's own profile) by running
     * [launch], which returns why the game couldn't be started, or null. A
     * session of key that is still open (the game was started again before the
     * watcher ended it) just goes on; one that is being ended is waited for
     * first.
     */
    suspend fun play(key: String, native: Boolean, title: String, pkg: String, launch: () -> String?): PlayOutcome {
        manager.awaitEnded(key)
        if (core.sessionOpen(key)) {
            watch(key, pkg, title)
            return launch()?.let { PlayOutcome.Failed(it) } ?: PlayOutcome.Started(false)
        }
        val out = begin(key, native, title).getOrElse { return PlayOutcome.Failed(it.message.orEmpty()) }
        if (out.cancelled) return PlayOutcome.Cancelled
        if (out.synced) watch(key, pkg, title)
        val error = launch()
        if (error != null) {
            if (out.synced) stopWatching(key)
            return PlayOutcome.Failed(error)
        }
        return PlayOutcome.Started(out.preOffline)
    }

    /**
     * Syncs and starts watching profile [key] whose game is already running
     * (the user came back to a game whose session had ended): as [play], but
     * nothing is started.
     */
    suspend fun track(key: String, native: Boolean, title: String, pkg: String): PlayOutcome {
        manager.awaitEnded(key)
        if (core.sessionOpen(key)) {
            watch(key, pkg, title)
            return PlayOutcome.Started(false)
        }
        val out = begin(key, native, title).getOrElse { return PlayOutcome.Failed(it.message.orEmpty()) }
        if (out.cancelled) return PlayOutcome.Cancelled
        if (out.synced) watch(key, pkg, title)
        return PlayOutcome.Started(out.preOffline)
    }

    private suspend fun begin(key: String, native: Boolean, title: String): Result<BeginOutcome> = try {
        Result.success(core.beginSession(key, if (native) NATIVE_PROFILE else ""))
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        Result.failure(IllegalStateException(startFailed(title, e.message ?: e.toString()), e))
    }
}
