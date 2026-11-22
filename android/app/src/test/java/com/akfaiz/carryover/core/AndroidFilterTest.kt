package com.akfaiz.carryover.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** What the Android app shows of the library: what can be played on Android. */
class AndroidFilterTest {
    private fun emu(name: String, preset: String = "", exe: String = "", setUp: Boolean = false) =
        EmulatorView(id = name.lowercase(), name = name, preset = preset, device = EmulatorDevice(exe = exe), setUp = setUp)

    @Test
    fun emulatorsWithAnAndroidApp() {
        assertTrue(EmulatorPresets.onAndroid(emu("Eden", preset = "switch")))
        assertTrue(EmulatorPresets.onAndroid(emu("RetroArch", preset = "retroarch")))
        assertTrue("NetherSX2 plays PCSX2's games", EmulatorPresets.onAndroid(emu("PCSX2", preset = "pcsx2")))
        // No preset: known by name.
        assertTrue(EmulatorPresets.onAndroid(emu("melonDS 1.0")))
        assertTrue(EmulatorPresets.onAndroid(emu("Citra")))
        // No Android app.
        assertFalse(EmulatorPresets.onAndroid(emu("Ryujinx", preset = "ryujinx")))
        assertFalse(EmulatorPresets.onAndroid(emu("Cemu", preset = "cemu")))
        assertFalse(EmulatorPresets.onAndroid(emu("RPCS3", preset = "rpcs3")))
        assertFalse(EmulatorPresets.onAndroid(emu("My Emu")))
        // Whatever it is, one set up on this device is used here.
        assertTrue(EmulatorPresets.onAndroid(emu("Ryujinx", preset = "ryujinx", setUp = true)))
        assertTrue(EmulatorPresets.onAndroid(emu("My Emu", exe = "com.example.emu")))
    }

    private val slots = listOf(SaveSlot("saves"))
    private fun profile(emuId: String) = ProfileView(emulatorId = emuId, name = emuId, key = "g.$emuId")

    @Test
    fun pcGamesAreLeftOut() {
        val pc = GameView(id = "g", name = "PC game", slots = slots)
        assertNull("a PC game's own saves don't sync with Android", pc.forAndroid(setOf("eden")))

        val emulated = GameView(id = "g", name = "Zelda", emulators = listOf(profile("eden"), profile("ryujinx")))
        val shown = emulated.forAndroid(setOf("eden"))!!
        assertEquals(listOf("eden"), shown.emulators.map { it.emulatorId })
        assertNull("only on an emulator without an Android app", emulated.forAndroid(setOf("retroarch")))

        // A PC game that is also emulated: only the emulator's saves.
        val both = GameView(id = "g", name = "Both", slots = slots, emulators = listOf(profile("eden")))
        assertEquals(listOf("g.eden"), both.forAndroid(setOf("eden"))!!.profiles.map { it.key })
    }

    @Test
    fun aGameRecoveredFromTheCloudShowsToBeSetUp() {
        // Brought back from a leftover: this device has an entry for it, with no folder or app yet.
        val recovered = GameView(id = "g", name = "Recovered", slots = slots, hasDevice = true)
        assertEquals(listOf("g"), recovered.forAndroid(emptySet())!!.profiles.map { it.key })
        assertNull("a PC game has no entry for this device", GameView(id = "g", name = "PC game", slots = slots).forAndroid(emptySet()))
        assertNull("an entry for a game with no saves of its own shows nothing", GameView(id = "g", name = "X", hasDevice = true).forAndroid(emptySet()))
    }

    @Test
    fun ownSavesSetUpHereStay() {
        val added = GameView(id = "g", name = "Added here", slots = slots, device = DeviceConfig(paths = mapOf("saves" to "/sdcard/x")))
        assertEquals(listOf("g"), added.forAndroid(emptySet())!!.profiles.map { it.key })
        val androidApp = GameView(id = "g", name = "Stardew", slots = slots, device = DeviceConfig(exe = "com.chucklefish.stardewvalley"))
        assertEquals(1, androidApp.forAndroid(emptySet())!!.profiles.size)
    }

    // The library's filters, as on the desktop: Ready to play, Needs attention.
    private val eden = EmulatorView(id = "eden", name = "Eden", device = EmulatorDevice(exe = "dev.eden"))
    private fun zelda(paths: Map<String, String> = mapOf("sram" to "/sdcard/z"), status: GameStatus = GameStatus()) = GameView(
        id = "z",
        name = "Zelda",
        emulators = listOf(
            ProfileView(
                emulatorId = "eden", name = "Eden", key = "z.eden", slots = listOf(SaveSlot("sram")),
                paths = paths, resolvedPaths = paths, status = status,
            ),
        ),
    )

    @Test
    fun readyToPlay() {
        assertTrue(zelda().ready(listOf(eden)))
        assertFalse("no save folder here", zelda(paths = emptyMap()).ready(listOf(eden)))
        assertFalse("the emulator has no app here", zelda().ready(listOf(eden.copy(device = EmulatorDevice()))))
        val own = GameView(id = "s", name = "Stardew", slots = slots, resolvedPaths = mapOf("saves" to "/x"), device = DeviceConfig(paths = mapOf("saves" to "/x")))
        assertFalse("no Android app chosen", own.ready(emptyList()))
        assertTrue(own.copy(device = own.device.copy(exe = "com.stardew")).ready(emptyList()))
    }

    @Test
    fun needsAttention() {
        val emus = listOf(eden)
        assertFalse("ready and synced", zelda(status = GameStatus(lastSyncAt = "2026-11-21T10:00:00Z")).needsAttention(emus))
        assertTrue("not set up here", zelda(paths = emptyMap()).needsAttention(emus))
        assertTrue(zelda(status = GameStatus(conflict = true)).needsAttention(emus))
        assertTrue(zelda(status = GameStatus(lastError = "offline")).needsAttention(emus))
        assertTrue("saves not uploaded", zelda(status = GameStatus(localChanges = true)).needsAttention(emus))
        assertFalse("never while it is played", zelda(status = GameStatus(running = true, localChanges = true)).needsAttention(emus))
    }
}
