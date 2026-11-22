package com.akfaiz.carryover.core

/** What the Android app shows of the core's library: only what plays on Android. */
object AndroidLibrary {
    /** The library as this device sees it: the emulators sorted by name, and the games that play here, sorted by name. */
    data class View(val games: List<GameView>, val emulators: List<EmulatorView>)

    /**
     * Only what can be played here: emulated games (on emulators with an
     * Android app) and games set up on this device; PC games are left out.
     */
    fun of(games: List<GameView>, emulators: List<EmulatorView>): View {
        val sorted = emulators.sortedBy { it.name.lowercase() }
        val here = sorted.filter(EmulatorPresets::onAndroid).map { it.id }.toSet()
        val mine = games.mapNotNull { it.forAndroid(here) }.sortedBy { it.name.lowercase() }
        return View(mine, sorted)
    }

    /**
     * The save-path preset of an emulator being set up: the one it already has,
     * else the one of its app's package (for suggesting save folders).
     */
    fun presetFor(known: EmulatorView?, exe: String): String =
        known?.preset?.ifEmpty { null }
            ?: EmulatorPresets.forPackage(exe.substringBefore('/'))?.manifestPreset.orEmpty()
}
