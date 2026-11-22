package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView

/** What a launch of [LaunchActivity] is for, decided from its extras and the library. */
sealed interface LaunchPlan {
    /** The launch says neither a game nor an app to start. */
    data object NothingToLaunch : LaunchPlan

    /** The launch names a game profile (a shortcut) that no longer exists. */
    data object GameGone : LaunchPlan

    /** A shortcut or a recent game: play [cand] as the Play button does. */
    data class Play(val cand: Forward.Candidate) : LaunchPlan

    /** A game that is running again after its session ended: sync and watch it, starting nothing. */
    data class Track(val cand: Forward.Candidate) : LaunchPlan

    /** A frontend asked for [pkg], which is not set up here as an emulator or a game's app. */
    data class Refused(val pkg: String) : LaunchPlan

    /**
     * A frontend started the emulator [pkg] through Carryover: [candidates] are
     * the profiles played with it, [match] the one its [roms] (game files) say
     * it is (null: the user is asked).
     */
    data class Frontend(
        val pkg: String,
        val target: String,
        val roms: List<String>,
        val candidates: List<Forward.Candidate>,
        val match: Forward.Candidate?,
    ) : LaunchPlan
}

/**
 * Decides what a launch asks for, without Android classes: a profile key
 * (shortcut), a track request, or a target app a frontend wants started. The
 * activity is exported, so any app can send these extras: a target that is not
 * set up here is refused, never forwarded.
 */
object LaunchPlanner {
    fun plan(
        key: String?,
        track: Boolean,
        target: String?,
        games: List<GameView>,
        emulators: List<EmulatorView>,
        ownPackage: String,
        /** The launch's data URI, if any, and its string extras: where the game file may be named. */
        data: String?,
        extras: Map<String, String>,
    ): LaunchPlan = when {
        key != null -> {
            val cand = games.flatMap { g -> g.profiles.map { Forward.Candidate(g, it) } }
                .firstOrNull { it.profile.key == key }
            when {
                cand == null -> LaunchPlan.GameGone
                track -> LaunchPlan.Track(cand)
                else -> LaunchPlan.Play(cand)
            }
        }
        target != null -> {
            val pkg = Forward.packageOf(target)
            if (pkg !in Forward.allowedTargets(games, emulators, ownPackage)) {
                LaunchPlan.Refused(pkg)
            } else {
                val roms = Forward.roms(data, extras)
                val candidates = Forward.candidates(games, emulators, pkg)
                LaunchPlan.Frontend(pkg, target, roms, candidates, Forward.match(candidates, roms))
            }
        }
        else -> LaunchPlan.NothingToLaunch
    }
}
