package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.SaveSlot
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** What a launch of LaunchActivity asks for, from its extras and the library. */
class LaunchPlannerTest {
    private val own = "com.akfaiz.carryover"
    private val ppsspp = EmulatorView(id = "e1", name = "PPSSPP", device = EmulatorDevice(exe = "org.ppsspp.ppsspp/.PpssppActivity"))

    private fun psp(id: String, name: String, rom: String = "") = GameView(
        id = id,
        name = name,
        emulators = listOf(ProfileView(emulatorId = "e1", name = "PPSSPP", key = "$id.e1", rom = rom)),
    )

    private val native = GameView(
        id = "n",
        name = "Stardew",
        slots = listOf(SaveSlot("saves")),
        device = DeviceConfig(exe = "com.chucklefish.stardewvalley"),
    )

    private val games = listOf(psp("a", "Game A", rom = "/sdcard/ROMs/a.iso"), psp("b", "Game B"), native)
    private val emulators = listOf(ppsspp)

    private fun plan(
        key: String? = null,
        track: Boolean = false,
        target: String? = null,
        data: String? = null,
        extras: Map<String, String> = emptyMap(),
    ) = LaunchPlanner.plan(key, track, target, games, emulators, own, data, extras)

    @Test
    fun aLaunchWithNothingInItAsksForNothing() {
        assertEquals(LaunchPlan.NothingToLaunch, plan())
    }

    @Test
    fun aShortcutPlaysItsProfile() {
        val p = plan(key = "a.e1") as LaunchPlan.Play
        assertEquals("Game A (PPSSPP)", p.cand.title)
    }

    @Test
    fun aGamesOwnProfileIsFoundByItsId() {
        val p = plan(key = "n") as LaunchPlan.Play
        assertEquals("Stardew", p.cand.title)
    }

    @Test
    fun aShortcutOfAGameThatIsGone() {
        assertEquals(LaunchPlan.GameGone, plan(key = "zzz"))
    }

    @Test
    fun trackIsOnlyForAProfileKey() {
        assertEquals("a.e1", (plan(key = "a.e1", track = true) as LaunchPlan.Track).cand.profile.key)
        assertEquals("a gone game is gone, tracked or not", LaunchPlan.GameGone, plan(key = "zzz", track = true))
        assertEquals("track without a key asks for nothing", LaunchPlan.NothingToLaunch, plan(track = true))
    }

    @Test
    fun theKeyWinsOverATarget() {
        val p = plan(key = "b.e1", target = "org.ppsspp.ppsspp")
        assertTrue(p is LaunchPlan.Play)
    }

    @Test
    fun aFrontendWhoseFileIsAKnownGameIsMatched() {
        val p = plan(target = "org.ppsspp.ppsspp/.PpssppActivity", data = "file:///sdcard/ROMs/a.iso") as LaunchPlan.Frontend
        assertEquals("org.ppsspp.ppsspp", p.pkg)
        assertEquals("org.ppsspp.ppsspp/.PpssppActivity", p.target)
        assertEquals("a.e1", p.match?.profile?.key)
        assertEquals(listOf("a.e1", "b.e1"), p.candidates.map { it.profile.key })
        assertEquals(listOf("file:///sdcard/ROMs/a.iso"), p.roms)
    }

    @Test
    fun theFileCanBeInAnExtraInstead() {
        val p = plan(target = "org.ppsspp.ppsspp", extras = mapOf("ROM" to "/sdcard/ROMs/a.iso", "LIBRETRO" to "core")) as LaunchPlan.Frontend
        assertEquals("a.e1", p.match?.profile?.key)
    }

    @Test
    fun anUnknownFileAsksTheUserWhichGameItIs() {
        val p = plan(target = "org.ppsspp.ppsspp", data = "file:///sdcard/ROMs/new.iso") as LaunchPlan.Frontend
        assertNull(p.match)
        assertEquals(2, p.candidates.size)
        assertEquals(listOf("file:///sdcard/ROMs/new.iso"), p.roms)
    }

    @Test
    fun aGamesOwnAppNeedsNoFile() {
        val p = plan(target = "com.chucklefish.stardewvalley") as LaunchPlan.Frontend
        assertEquals("n", p.match?.profile?.key)
    }

    @Test
    fun anAppThatIsNotSetUpHereIsRefused() {
        assertEquals(LaunchPlan.Refused("com.evil.app"), plan(target = "com.evil.app/.Main"))
    }

    @Test
    fun carryoverItselfIsNeverATarget() {
        // The activity is exported: no caller may use it to start Carryover's own screens.
        assertEquals(LaunchPlan.Refused(own), plan(target = "$own/.MainActivity"))
        val withSelfAsEmulator = LaunchPlanner.plan(
            null, false, "$own/.MainActivity", games,
            emulators + EmulatorView(id = "self", name = "Self", device = EmulatorDevice(exe = own)),
            own, null, emptyMap(),
        )
        assertEquals(LaunchPlan.Refused(own), withSelfAsEmulator)
    }

    @Test
    fun anEmptyTargetIsRefused() {
        assertEquals(LaunchPlan.Refused(""), plan(target = ""))
    }
}
