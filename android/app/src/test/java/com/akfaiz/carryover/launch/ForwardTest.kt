package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.SaveSlot
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ForwardTest {
    private val storage = "content://com.android.externalstorage.documents"

    @Test
    fun normalize() {
        assertEquals("/storage/emulated/0/ROMs/a.iso", Forward.normalize("/storage/emulated/0/ROMs/a.iso"))
        assertEquals("/storage/emulated/0/ROMs/My Game.iso", Forward.normalize("file:///storage/emulated/0/ROMs/My%20Game.iso"))
        // ES-DE's %ROMSAF%: a document inside a granted tree.
        assertEquals(
            "/storage/emulated/0/ROMs/psp/a.iso",
            Forward.normalize("$storage/tree/primary%3AROMs/document/primary%3AROMs%2Fpsp%2Fa.iso"),
        )
        // The file picker's document URI.
        assertEquals("/storage/emulated/0/ROMs/psp/a.iso", Forward.normalize("$storage/document/primary%3AROMs%2Fpsp%2Fa.iso"))
        assertEquals("/storage/1234-ABCD/a.iso", Forward.normalize("$storage/document/1234-ABCD%3Aa.iso"))
        assertEquals("content:org.emu.files:root:a.bin", Forward.normalize("content://org.emu.files/document/root%3Aa.bin"))
        assertNull(Forward.normalize("snes9x"))
        assertNull(Forward.normalize("true"))
    }

    @Test
    fun sameFile() {
        assertTrue(Forward.sameFile("/sdcard/ROMs/a.iso", "$storage/tree/primary%3AROMs/document/primary%3AROMs%2Fa.iso"))
        assertTrue(Forward.sameFile("/storage/self/primary/ROMs/a.iso", "file:///storage/emulated/0/ROMs/a.iso"))
        assertFalse(Forward.sameFile("/sdcard/ROMs/a.iso", "/sdcard/ROMs/b.iso"))
        assertFalse(Forward.sameFile("snes9x", "snes9x"))
    }

    @Test
    fun romsSkipsCarryoverExtrasAndNonFiles() {
        val roms = Forward.roms(
            "content://x/document/a",
            mapOf("ROM" to "/sdcard/a.sfc", "LIBRETRO" to "snes9x", Forward.EXTRA_TARGET to "/not/a/rom"),
        )
        assertEquals(listOf("content://x/document/a", "/sdcard/a.sfc"), roms)
    }

    private val ppsspp = EmulatorView(id = "e1", name = "PPSSPP", device = EmulatorDevice(exe = "org.ppsspp.ppsspp/.PpssppActivity"))
    private val dolphin = EmulatorView(id = "e2", name = "Dolphin", device = EmulatorDevice(exe = "org.dolphinemu.dolphinemu"))

    private fun game(id: String, name: String, vararg profiles: ProfileView, app: String = "", ownSlots: Boolean = false) = GameView(
        id = id,
        name = name,
        slots = if (ownSlots) listOf(SaveSlot("saves")) else emptyList(),
        device = DeviceConfig(exe = app),
        emulators = profiles.toList(),
    )

    private fun profile(emu: EmulatorView, game: String, rom: String = "") =
        ProfileView(emulatorId = emu.id, name = emu.name, key = "$game.${emu.id}", rom = rom)

    @Test
    fun candidatesAndMatch() {
        val a = game("a", "Game A", profile(ppsspp, "a", rom = "/sdcard/ROMs/a.iso"), profile(dolphin, "a"))
        val b = game("b", "Game B", profile(ppsspp, "b"))
        val native = game("n", "Native", app = "com.example.game", ownSlots = true)
        val games = listOf(a, b, native)
        val emus = listOf(ppsspp, dolphin)

        val psp = Forward.candidates(games, emus, "org.ppsspp.ppsspp")
        assertEquals(listOf("a.e1", "b.e1"), psp.map { it.profile.key })
        assertEquals("Game A (PPSSPP)", psp[0].title)
        assertEquals("a.e1", Forward.match(psp, listOf("$storage/tree/x/document/primary%3AROMs%2Fa.iso"))?.profile?.key)
        assertNull("unknown file: ask", Forward.match(psp, listOf("/sdcard/ROMs/other.iso")))

        val own = Forward.candidates(games, emus, "com.example.game")
        assertEquals(listOf("n"), own.map { it.profile.key })
        assertEquals("a game's own app needs no file", "n", Forward.match(own, emptyList())?.profile?.key)
        assertTrue(Forward.candidates(games, emus, "").isEmpty())
    }

    @Test
    fun allowedTargets() {
        val games = listOf(game("n", "Native", app = "com.example.game", ownSlots = true))
        val allowed = Forward.allowedTargets(games, listOf(ppsspp, dolphin, EmulatorView(id = "x")), "com.akfaiz.carryover")
        assertEquals(setOf("org.ppsspp.ppsspp", "org.dolphinemu.dolphinemu", "com.example.game"), allowed)
        val self = Forward.allowedTargets(emptyList(), listOf(EmulatorView(device = EmulatorDevice(exe = "com.akfaiz.carryover"))), "com.akfaiz.carryover")
        assertTrue("Carryover itself is never a target", self.isEmpty())
    }

    @Test
    fun mayGrant() {
        val saves = "$storage/tree/primary%3AEmu%2Fsaves"
        val rom = "$storage/tree/primary%3AROMs/document/primary%3AROMs%2Fa.iso"
        assertTrue(Forward.mayGrant(rom, grantedByCaller = true, persistedTrees = listOf(saves)))
        assertFalse("not granted with the launch", Forward.mayGrant(rom, grantedByCaller = false, persistedTrees = emptyList()))
        assertFalse(Forward.mayGrant("/sdcard/a.iso", grantedByCaller = true, persistedTrees = emptyList()))
        // Carryover's own save folders, in any URI form, are never passed on.
        assertFalse(Forward.mayGrant(saves, true, listOf(saves)))
        assertFalse(Forward.mayGrant("$saves/document/primary%3AEmu%2Fsaves%2Fa.sav", true, listOf(saves)))
        assertFalse(Forward.mayGrant("$storage/document/primary%3AEmu%2Fsaves%2Fa.sav", true, listOf(saves)))
        assertTrue(Forward.mayGrant("$storage/document/primary%3AEmu%2Fsaves2%2Fa.sav", true, listOf(saves)))
    }
}
