package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.Profile
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/** What starts a profile on this device: its app, the launch arguments and the game file. */
class SessionsSetupTest {
    private fun profile(emulatorId: String? = null, app: String = "", rom: String = "") = Profile(
        key = "k",
        emulator = emulatorId,
        emulatorId = emulatorId,
        slots = emptyList(),
        paths = emptyMap(),
        resolvedPaths = emptyMap(),
        pathErrors = emptyMap(),
        status = GameStatus(),
        rom = rom,
        app = app,
    )

    private val eden = EmulatorView(id = "eden", name = "Eden", device = EmulatorDevice(exe = "dev.eden.eden_emulator/.Main", args = "-a VIEW -d {rom}"))

    @Test
    fun aGamesOwnAppIsItsPackageWithNoArguments() {
        assertEquals(Triple("com.chucklefish.stardewvalley", "", ""), Sessions.setup(profile(app = "com.chucklefish.stardewvalley"), emptyList()))
    }

    @Test
    fun aGameWithNoAppChosenHasNoSetup() {
        assertNull(Sessions.setup(profile(), emptyList()))
        assertNull(Sessions.setup(profile(app = "   "), emptyList()))
    }

    @Test
    fun anEmulatorProfileUsesTheEmulatorsAppAndItsOwnGameFile() {
        val s = Sessions.setup(profile(emulatorId = "eden", rom = "content://x/doc/zelda.nsp"), listOf(eden))
        assertEquals(Triple("dev.eden.eden_emulator/.Main", "-a VIEW -d {rom}", "content://x/doc/zelda.nsp"), s)
    }

    @Test
    fun anEmulatorNotSetUpHereHasNoSetup() {
        assertNull(Sessions.setup(profile(emulatorId = "eden"), emptyList()))
        assertNull("no app chosen for it on this device", Sessions.setup(profile(emulatorId = "eden"), listOf(eden.copy(device = EmulatorDevice()))))
    }

    @Test
    fun theGameFileComesFromTheProfileNotTheEmulator() {
        val a = Sessions.setup(profile(emulatorId = "eden", rom = "a"), listOf(eden))!!
        val b = Sessions.setup(profile(emulatorId = "eden", rom = "b"), listOf(eden))!!
        assertEquals("a", a.third)
        assertEquals("b", b.third)
        assertEquals(a.first, b.first)
    }
}
