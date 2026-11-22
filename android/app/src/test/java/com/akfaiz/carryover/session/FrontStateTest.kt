package com.akfaiz.carryover.session

import com.akfaiz.carryover.session.FrontState.Verdict
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class FrontStateTest {
    private val game = "org.emu.game"
    private val t0 = 1_000_000L

    private fun state() = FrontState(game, startedAt = t0)

    @Test
    fun keepsWhileTheGameIsInFront() {
        val s = state()
        s.onFront(game, t0 + 500)
        assertEquals(Verdict.KEEP, s.verdict(t0 + 120_000, hasUsageAccess = true))
    }

    @Test
    fun endsWhenAnotherAppStaysInFrontLongerThanTheGrace() {
        val s = state()
        s.onFront(game, t0 + 500)
        s.onFront("com.android.launcher", t0 + 20_000)
        assertEquals(Verdict.KEEP, s.verdict(t0 + 29_999, true))
        assertEquals(Verdict.LEFT, s.verdict(t0 + 30_000, true))
    }

    @Test
    fun comingBackWithinTheGraceKeepsTheSession() {
        val s = state()
        s.onFront(game, t0 + 500)
        s.onFront("com.android.launcher", t0 + 20_000)
        s.onFront(game, t0 + 25_000)
        assertEquals(Verdict.KEEP, s.verdict(t0 + 200_000, true))
    }

    @Test
    fun aGameLeftWithinOneCheckStillCountsAsLeftWhenItWas() {
        // Both events arrive in one query: the game, then the launcher.
        val s = state()
        s.onFront(game, t0 + 1_000)
        s.onFront("com.android.launcher", t0 + 3_000)
        assertEquals(t0 + 3_000, s.leftAt)
        assertEquals(Verdict.LEFT, s.verdict(t0 + 13_000, true))
    }

    @Test
    fun theGracePeriodIsAParameter() {
        val s = state()
        s.onFront(game, t0 + 500)
        s.onFront("com.android.launcher", t0 + 10_000)
        assertEquals(Verdict.LEFT, s.verdict(t0 + 20_000, true, graceMs = 10_000))
        assertEquals(Verdict.KEEP, s.verdict(t0 + 20_000, true, graceMs = 60_000))
        assertEquals(Verdict.LEFT, s.verdict(t0 + 70_000, true, graceMs = 60_000))
    }

    @Test
    fun systemScreensOverTheGameDontCountAsLeaving() {
        val s = state()
        s.onFront(game, t0 + 500)
        for (pkg in listOf("com.google.android.gms", "com.android.permissioncontroller", "com.google.android.documentsui")) {
            s.onFront(pkg, t0 + 5_000)
        }
        assertEquals(null, s.leftAt)
        assertEquals(Verdict.KEEP, s.verdict(t0 + 100_000, true))
        assertTrue(FrontPolicy.SYSTEM_SCREENS.contains("android"))
    }

    @Test
    fun aSystemScreenAfterLeavingDoesNotRestartTheGrace() {
        val s = state()
        s.onFront(game, t0 + 500)
        s.onFront("com.android.launcher", t0 + 5_000)
        s.onFront("com.google.android.gms", t0 + 12_000)
        assertEquals(t0 + 5_000, s.leftAt)
        assertEquals(Verdict.LEFT, s.verdict(t0 + 15_000, true))
    }

    @Test
    fun anotherAppBeforeTheGameCameToFrontIsNotLeavingIt() {
        // The launcher is in front while the game starts up.
        val s = state()
        s.onFront("com.android.launcher", t0 + 100)
        assertEquals(null, s.leftAt)
        s.onFront(game, t0 + 2_000)
        assertEquals(Verdict.KEEP, s.verdict(t0 + 100_000, true))
    }

    @Test
    fun eventsFromBeforeTheSessionAreIgnored() {
        val s = state()
        s.onFront(game, t0 - 60_000) // an earlier run of the game
        assertEquals(false, s.everFront)
        s.onFront(game, t0 - 1_000) // within the clock slack
        assertEquals(true, s.everFront)
    }

    @Test
    fun aGameThatNeverComesToTheFrontEndsOnlyWithUsageAccess() {
        val s = state()
        assertEquals(Verdict.KEEP, s.verdict(t0 + 59_000, true))
        assertEquals(Verdict.NEVER_STARTED, s.verdict(t0 + 61_000, true))
        assertEquals(Verdict.KEEP, s.verdict(t0 + 61_000, false))
    }

    @Test
    fun aPickedUpSessionIsTreatedAsInFrontUntilAnotherAppIs() {
        val s = FrontState(game, startedAt = t0 - FrontPolicy.NEVER_STARTED_MS, everFront = true)
        assertEquals(Verdict.KEEP, s.verdict(t0, true))
        s.onFront("com.android.launcher", t0 + 1_000)
        assertEquals(Verdict.LEFT, s.verdict(t0 + 11_000, true))
    }
}
