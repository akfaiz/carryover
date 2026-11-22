package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.OpenSession
import com.akfaiz.carryover.core.SessionCore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.getAndUpdate
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.ConcurrentHashMap

/** A screen that came to the front: its app's package, and when. */
data class FrontEvent(val pkg: String, val at: Long)

/** Which app is in front: Usage access, behind an interface so the manager runs without Android. */
interface FrontSource {
    fun hasUsageAccess(): Boolean

    /** The screens that came to the front between [from] and [to], oldest first. */
    fun events(from: Long, to: Long): List<FrontEvent>
}

/** What the manager tells the user (notifications on Android). */
interface SessionNotices {
    /** How ending the session of [key] went. */
    fun result(title: String, key: String, result: Result<EndOutcome>)

    /** The game of [key], whose session ended, came back to the front: offer to track it again. */
    fun offerTracking(key: String, title: String)
    fun cancelOffer(key: String)
}

/** The foreground service the manager runs in (a fake in tests). */
interface SessionHost {
    /** What the manager is doing changed: refresh the ongoing notification. */
    fun changed()

    /** Nothing is left to watch, end or wait for: the service can stop. */
    fun idle()
}

/**
 * The play sessions of this process: which are watched and how long a game
 * has been out of front ([FrontState]), which are being ended (uploaded), and
 * which games that were left may come back. No Android classes: the service,
 * the notifications and the Usage-access query are [SessionHost],
 * [SessionNotices] and [FrontSource], so the rules (when a session ends, what
 * a failed upload does, what `recover` picks up) are unit tested with a fake
 * [SessionCore].
 *
 * Ending is done on [scope], not in the service: an upload goes on if the
 * service is destroyed meanwhile.
 */
class SessionManager(
    private val core: SessionCore,
    private val notices: SessionNotices,
    private val front: FrontSource,
    private val scope: CoroutineScope,
    /** The user's grace period before a game that was left ends its session. */
    private val graceMs: () -> Long,
    /** Asks for the upload of a failed or offline end to be tried again when online. */
    private val retryUploads: () -> Unit,
    private val log: (String) -> Unit = {},
    private val now: () -> Long = System::currentTimeMillis,
) {
    private class Watch(val key: String, val title: String, val front: FrontState) {
        val pkg: String get() = front.pkg
    }

    /** A game whose session ended because the user left it: if it comes back, it is offered to be tracked again. */
    private class Recent(val key: String, val pkg: String, val title: String, val endedAt: Long)

    /** What the ongoing notification shows. */
    data class Snapshot(val playing: List<String>, val uploading: Boolean, val waiting: List<String>)

    private val watches = ConcurrentHashMap<String, Watch>()
    private val recent = ConcurrentHashMap<String, Recent>()
    private var lastQuery = 0L
    private var frontPkg: String? = null

    @Volatile
    private var host: SessionHost? = null

    private val _watching = MutableStateFlow<Set<String>>(emptySet())

    /** Keys of the sessions being watched. */
    val watching: StateFlow<Set<String>> = _watching
    private val _ending = MutableStateFlow<Set<String>>(emptySet())

    /** Keys of the sessions being ended (uploading). */
    val ending: StateFlow<Set<String>> = _ending

    fun attach(host: SessionHost) {
        this.host = host
    }

    fun detach(host: SessionHost) {
        if (this.host === host) this.host = null
    }

    /** Whether there is anything to poll for. */
    val hasWork: Boolean get() = watches.isNotEmpty() || recent.isNotEmpty()

    private val isIdle: Boolean get() = watches.isEmpty() && _ending.value.isEmpty() && recent.isEmpty()

    fun snapshot() = Snapshot(
        playing = watches.values.map { it.title },
        uploading = _ending.value.isNotEmpty(),
        waiting = recent.values.map { it.title },
    )

    /** The poll starts: events from just before (the game's start) on are looked at. */
    fun startPolling() {
        lastQuery = now() - FIRST_LOOKBACK_MS
    }

    // Watching

    /** Watches the session of [key], whose game runs in [pkg]. */
    fun watch(key: String, pkg: String, title: String) {
        if (key.isEmpty()) return
        watches[key] = Watch(key, title, FrontState(pkg, now()))
        recent.remove(key)
        notices.cancelOffer(key)
        log("watching $key ($pkg)")
        publish()
    }

    /** One look at the games watched: ends the sessions whose game was left or never started. */
    fun check() {
        val t = now()
        val usage = front.hasUsageAccess()
        if (usage) {
            // Every screen that came to the front since the last check, in
            // order: a game that came to the front and was left (or closed)
            // within one check still counts as played, and left when it was.
            for (e in front.events(lastQuery, t)) {
                frontPkg = e.pkg
                for (w in watches.values) w.front.onFront(e.pkg, e.at)
                offerTracking(e.pkg, e.at)
            }
            lastQuery = t
        }
        val grace = graceMs()
        for (w in watches.values) {
            when (w.front.verdict(t, usage, grace)) {
                FrontState.Verdict.LEFT -> {
                    log("${w.key}: left ${w.pkg} for $frontPkg")
                    end(w.key, rememberForTracking = true)
                }
                FrontState.Verdict.NEVER_STARTED -> {
                    log("${w.key}: ${w.pkg} never came to the front")
                    end(w.key)
                }
                FrontState.Verdict.KEEP -> {}
            }
        }
        // The wait for a game to come back ran out: nothing left to watch.
        if (recent.values.removeAll { t - it.endedAt > RECENT_MS } && recent.isEmpty()) stopIfIdle()
    }

    /** A game whose session ended came back to the front: offer to track its saves again. */
    private fun offerTracking(pkg: String, at: Long) {
        val r = recent.values.firstOrNull { it.pkg == pkg && at > it.endedAt } ?: return
        recent.remove(r.key)
        stopIfIdle() // no longer waiting for it (stays up if something else is watched)
        if (watches.containsKey(r.key) || _ending.value.contains(r.key)) return
        notices.offerTracking(r.key, r.title)
    }

    // Ending

    /** Ends the session of [key] now (null: every watched one): the user said they are done. */
    fun done(key: String?) {
        if (key != null) end(key) else watches.keys.toList().forEach { end(it) }
    }

    /**
     * Ends the session of [key] (upload, release the cloud lock) and tells the
     * user. A second request while it is being ended does nothing: it would
     * only fail against the first one's lock and report a failed upload.
     * [rememberForTracking]: the user left the game, so if it comes back it is
     * offered to be tracked again.
     */
    fun end(key: String, rememberForTracking: Boolean = false) {
        val w = watches.remove(key)
        if (key in _ending.getAndUpdate { it + key }) return
        val title = w?.title ?: key
        if (rememberForTracking && w != null && w.pkg.isNotEmpty()) {
            recent[key] = Recent(key, w.pkg, title, now())
        }
        publish()
        scope.launch {
            val result = try {
                Result.success(core.endSession(key))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                log("end $key failed: ${e.message}")
                Result.failure(e)
            }
            notices.result(title, key, result)
            // The upload didn't happen: try again when there is a network.
            if (result.isFailure || result.getOrNull()?.offline == true) retryUploads()
            _ending.update { it - key }
            stopIfIdle()
        }
    }

    /**
     * Waits until the session of [key] that is being ended (uploading) is over.
     * Starting the game again meanwhile would let that upload release the new
     * session's cloud lock and remove its marker, and read saves the game is
     * writing: the new session would end up untracked.
     */
    suspend fun awaitEnded(key: String, timeoutMs: Long = UPLOAD_WAIT_MS) {
        if (key !in _ending.value) return
        withTimeoutOrNull(timeoutMs) { _ending.first { key !in it } }
    }

    /**
     * Back in Carryover. Without Usage access that is the sign the games
     * watched were left (the ones just started excepted: the user is still on
     * the way to them). Also picks up open sessions of a killed process.
     */
    suspend fun appResumed() {
        recover(afterBoot = false)
        if (front.hasUsageAccess()) return
        val t = now()
        watches.values.filter { t - it.front.startedAt > JUST_STARTED_MS }.forEach {
            log("${it.key}: back in Carryover")
            end(it.key)
        }
    }

    /**
     * Picks up open sessions this process doesn't know (it was killed while a
     * game ran). After a reboot no game runs, so they end at once.
     */
    suspend fun recover(afterBoot: Boolean) {
        val open: List<OpenSession> = runCatching { core.openSessions() }.getOrDefault(emptyList())
        val games = if (open.isEmpty()) emptyList() else runCatching { core.games() }.getOrDefault(emptyList())
        val emulators = if (open.isEmpty()) emptyList() else runCatching { core.emulators() }.getOrDefault(emptyList())
        val t = now()
        for (s in open) {
            if (watches.containsKey(s.key) || _ending.value.contains(s.key)) continue
            val pkg = s.app(games, emulators)
            val title = s.title(games, emulators)
            if (afterBoot || pkg.isEmpty()) {
                log("ending open session ${s.key}" + if (afterBoot) " after a reboot" else " (no app known)")
                watches[s.key] = Watch(s.key, title, FrontState(pkg, t, everFront = true, leftAt = t))
                end(s.key)
            } else {
                // It may still be running: treat it as in front until another app is.
                watches[s.key] = Watch(s.key, title, FrontState(pkg, t - FrontPolicy.NEVER_STARTED_MS, everFront = true))
                log("picked up open session ${s.key} ($pkg)")
            }
        }
        publish()
        stopIfIdle()
    }

    private fun publish() {
        _watching.value = watches.keys.toSet()
        host?.changed()
    }

    private fun stopIfIdle() {
        if (isIdle) host?.idle() else publish()
    }

    companion object {
        /** How long a new session waits for the upload of the previous one of the same game. */
        const val UPLOAD_WAIT_MS = 5 * 60_000L

        /**
         * How long after its session ended the manager keeps waiting for the
         * game to come back to the front, to offer tracking it again.
         */
        const val RECENT_MS = 10 * 60_000L
        const val JUST_STARTED_MS = 5_000L
        private const val FIRST_LOOKBACK_MS = 10_000L

        private var instance: SessionManager? = null

        /** The process's manager, set by the Application (see [install]). */
        val shared: SessionManager get() = checkNotNull(instance) { "SessionManager not installed" }

        fun install(manager: SessionManager) {
            instance = manager
        }
    }
}

/** The package of the app that plays this session ("" when unknown). */
fun OpenSession.app(games: List<GameView>, emulators: List<EmulatorView>): String {
    val exe = if (emulator.isEmpty()) {
        games.firstOrNull { it.id == game }?.device?.exe
    } else {
        emulators.firstOrNull { it.id == emulator }?.device?.exe
    }
    return exe?.substringBefore('/').orEmpty()
}

/** What the user calls this session: "Game", or "Game (Emulator)"; the key when neither is known. */
fun OpenSession.title(games: List<GameView>, emulators: List<EmulatorView>): String {
    val name = games.firstOrNull { it.id == game }?.name.orEmpty()
    val emu = emulators.firstOrNull { it.id == emulator }?.name
    return (if (emu == null) name else "$name ($emu)").ifEmpty { key }
}
