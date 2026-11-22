package com.akfaiz.carryover.core

import org.junit.Assert.assertEquals
import org.junit.Test

/** What the app shows of the core's library, in which order. */
class AndroidLibraryTest {
    private val eden = EmulatorView(id = "eden", name = "Eden", preset = "switch")
    private val retro = EmulatorView(id = "ra", name = "retroarch", preset = "retroarch")
    private val ryujinx = EmulatorView(id = "ryu", name = "Ryujinx", preset = "ryujinx")

    private fun emulated(id: String, name: String, vararg emu: String) = GameView(
        id = id,
        name = name,
        emulators = emu.map { ProfileView(emulatorId = it, name = it, key = "$id.$it") },
    )

    @Test
    fun gamesAreSortedByNameWithoutCaseAndPcGamesLeftOut() {
        val games = listOf(
            emulated("1", "zelda", "eden"),
            emulated("2", "Banjo", "eden"),
            emulated("3", "Pc only", "ryu"),
            GameView(id = "4", name = "PC game", slots = listOf(SaveSlot("saves"))),
            emulated("5", "alpha", "ra"),
        )
        val v = AndroidLibrary.of(games, listOf(eden, retro, ryujinx))
        assertEquals(listOf("alpha", "Banjo", "zelda"), v.games.map { it.name })
    }

    @Test
    fun emulatorsAreSortedByNameWithoutCase() {
        val v = AndroidLibrary.of(emptyList(), listOf(ryujinx, eden, retro))
        assertEquals(listOf("Eden", "retroarch", "Ryujinx"), v.emulators.map { it.name })
    }

    @Test
    fun aGameKeepsOnlyItsAndroidProfiles() {
        val v = AndroidLibrary.of(listOf(emulated("1", "Zelda", "eden", "ryu")), listOf(eden, ryujinx))
        assertEquals(listOf("1.eden"), v.games.single().profiles.map { it.key })
    }

    @Test
    fun anEmulatorSetUpHereIsUsedWhateverItIs() {
        val setUp = ryujinx.copy(setUp = true)
        val v = AndroidLibrary.of(listOf(emulated("1", "Zelda", "ryu")), listOf(setUp))
        assertEquals(1, v.games.size)
    }

    @Test
    fun anEmptyLibraryIsEmpty() {
        val v = AndroidLibrary.of(emptyList(), emptyList())
        assertEquals(emptyList<GameView>(), v.games)
        assertEquals(emptyList<EmulatorView>(), v.emulators)
    }

    @Test
    fun thePresetOfAnEmulatorBeingSetUp() {
        // Its own preset first.
        assertEquals("switch", AndroidLibrary.presetFor(eden, "org.ppsspp.ppsspp"))
        // Else the one of its app's package: PPSSPP's own, with or without an activity.
        val preset = EmulatorPresets.all.first { it.manifestPreset.isNotEmpty() }
        val pkg = preset.packages.first()
        assertEquals(preset.manifestPreset, AndroidLibrary.presetFor(null, pkg))
        assertEquals(preset.manifestPreset, AndroidLibrary.presetFor(EmulatorView(id = "x"), "$pkg/.Main"))
        // An unknown app has none.
        assertEquals("", AndroidLibrary.presetFor(null, "com.example.unknown"))
    }
}
