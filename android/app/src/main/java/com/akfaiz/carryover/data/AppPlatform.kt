package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.session.PlayOutcome
import kotlinx.coroutines.flow.StateFlow
import java.io.File

/**
 * What the view models need of Android itself: starting a game, the session
 * service, home-screen shortcuts, covers and logs. No Android types in it, so
 * the view models run in unit tests with a fake; [com.akfaiz.carryover.AndroidPlatform]
 * is the real one.
 */
interface AppPlatform {
    /** Keys of the play sessions being ended (uploading). */
    val ending: StateFlow<Set<String>>

    /**
     * Plays profile [key] of [p] ([native]: the game's own app): syncs, takes
     * the cloud lock, starts the app and hands the session to the watcher.
     * A failure means the game can't be started on this device (its message
     * says why); how the session itself went is the [PlayOutcome].
     */
    suspend fun play(key: String, native: Boolean, title: String, p: Profile, emulators: List<EmulatorView>): Result<PlayOutcome>

    /** The game was started: it is one of the app icon's shortcuts. */
    fun rememberRecent(key: String, title: String, coverPath: String)

    /** Pins a home-screen shortcut that plays [key]; false when the launcher can't. */
    fun pinShortcut(key: String, title: String, coverPath: String): Boolean

    /** Ends the play session of [key] now (the user is done). */
    fun donePlaying(key: String)

    /** Back in the foreground: let the session watcher look at the open sessions. */
    fun appResumed()

    /** An image the user picked ([uri], a content:// URI) as a cover file; the caller deletes it. */
    suspend fun prepareCover(uri: String): File

    /** The system lets the app ask to install an APK ("Install unknown apps"). */
    fun canInstallPackages(): Boolean

    /** Opens the system page where that is allowed; false when there is none. */
    fun openInstallPermission(): Boolean

    /** Hands the APK at [path] to the system installer, which asks the user; false when it can't be opened. */
    fun installApk(path: String): Boolean

    /** The logs for a bug report, as a zip. */
    suspend fun buildLogs(): File

    /** Copies a zip from [buildLogs] to the document at [target] (a content:// URI). */
    suspend fun saveLogs(zip: File, target: String)
}
