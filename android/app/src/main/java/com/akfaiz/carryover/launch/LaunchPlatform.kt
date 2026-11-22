package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.session.PlayOutcome

/** A launch of [LaunchActivity] as data: what its intent said, without the intent. */
data class LaunchRequest(
    /** A game profile to play (shortcuts), or null. */
    val key: String?,
    /** The game is already running: sync and watch it, start nothing. */
    val track: Boolean,
    /** The app a frontend wants started (package or package/activity), or null. */
    val target: String?,
    /** The launch's data URI, if any, and its string extras: where the game file may be named. */
    val data: String?,
    val extras: Map<String, String>,
)

/**
 * What a launch needs of Android: the intent that came in, the app labels,
 * starting the game. No Android types in it, so [LaunchViewModel] runs in unit
 * tests with a fake; [AndroidLaunchPlatform] is the real one, made for one
 * incoming intent.
 */
interface LaunchPlatform {
    val request: LaunchRequest

    /** Carryover's own package: never a target. */
    val ownPackage: String

    /** The name of an installed app, or its package when it has none. */
    fun appLabel(pkg: String): String

    /**
     * Prepares starting [target] with this launch's own intent, as the
     * frontend sent it (see [AndroidLaunchPlatform]); false when [target]
     * isn't an exported activity of that app.
     */
    fun prepareForward(target: String): Boolean

    /**
     * Plays [cand] by what its profile is set up with. A failure means the
     * game can't be started on this device (its message says why).
     */
    suspend fun play(cand: Forward.Candidate, emulators: List<EmulatorView>): Result<PlayOutcome>

    /** Plays [cand] by starting the prepared launch of the app [pkg] ([prepareForward]). */
    suspend fun playForward(cand: Forward.Candidate, pkg: String): PlayOutcome

    /** Syncs and watches [cand], whose game (the app [pkg]) is already running. */
    suspend fun track(cand: Forward.Candidate, pkg: String): PlayOutcome

    /** Starts the prepared launch without syncing; why it couldn't be, or null. */
    fun startWithoutSync(): String?

    /** The game was started: it is one of the app icon's shortcuts. */
    fun rememberRecent(cand: Forward.Candidate)

    fun log(what: String, e: Throwable)
}
