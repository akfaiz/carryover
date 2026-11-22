package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Notice
import com.akfaiz.carryover.core.NoticeKind
import com.akfaiz.carryover.core.OpenSession
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** The rules of the session watcher, against a fake core, clock and Usage access. */
class SessionManagerTest {
    private val core = FakeCore()
    private val notices = FakeNotices()
    private val front = FakeFront()
    private val host = FakeHost()
    private var clock = 0L
    private var retries = 0
    private var grace = 10_000L

    // Unconfined: with a fake core that doesn't suspend, an end finishes inside the call.
    private val manager = SessionManager(
        core = core,
        notices = notices,
        front = front,
        scope = CoroutineScope(Dispatchers.Unconfined),
        graceMs = { grace },
        retryUploads = { retries++ },
        now = { clock },
    ).also { it.attach(host) }

    private fun at(t: Long) {
        clock = t
    }

    @Test
    fun aGameThatWasLeftEndsAfterTheGracePeriod() {
        manager.watch("g", "pkg.game", "Game")
        front.events += FrontEvent("pkg.game", 500)
        at(1_000)
        manager.check()
        front.events += FrontEvent("other.app", 2_000)
        at(2_500)
        manager.check()
        assertEquals("inside the grace period", emptyList<String>(), core.ended)

        at(12_100)
        manager.check()
        assertEquals(listOf("g"), core.ended)
        assertEquals(listOf("Game"), notices.results.map { it.first })
    }

    @Test
    fun comingBackWithinTheGracePeriodKeepsTheSession() {
        manager.watch("g", "pkg.game", "Game")
        front.events += FrontEvent("pkg.game", 500)
        front.events += FrontEvent("other.app", 2_000)
        front.events += FrontEvent("pkg.game", 6_000)
        at(30_000)
        manager.check()
        assertEquals(emptyList<String>(), core.ended)
    }

    @Test
    fun aSystemScreenOverTheGameIsNotLeavingIt() {
        manager.watch("g", "pkg.game", "Game")
        front.events += FrontEvent("pkg.game", 500)
        front.events += FrontEvent("com.android.permissioncontroller", 2_000)
        at(60_000)
        manager.check()
        assertEquals(emptyList<String>(), core.ended)
    }

    @Test
    fun theGracePeriodIsTheUsersSetting() {
        grace = 30_000
        manager.watch("g", "pkg.game", "Game")
        front.events += FrontEvent("pkg.game", 500)
        front.events += FrontEvent("other.app", 2_000)
        at(20_000)
        manager.check()
        assertEquals(emptyList<String>(), core.ended)
        at(32_100)
        manager.check()
        assertEquals(listOf("g"), core.ended)
    }

    @Test
    fun aGameThatNeverStartedEnds() {
        manager.watch("g", "pkg.game", "Game")
        at(61_000)
        manager.check()
        assertEquals(listOf("g"), core.ended)
        assertEquals("nothing to wait for: the service can stop", 1, host.idle)
    }

    @Test
    fun withoutUsageAccessNothingEndsByItself() {
        front.usage = false
        manager.watch("g", "pkg.game", "Game")
        at(10 * 60_000)
        manager.check()
        assertEquals(emptyList<String>(), core.ended)
    }

    @Test
    fun doneEndsOneOrAllWatchedSessions() {
        manager.watch("a", "pkg.a", "A")
        manager.watch("b", "pkg.b", "B")
        manager.done("a")
        assertEquals(listOf("a"), core.ended)
        manager.done(null)
        assertEquals(listOf("a", "b"), core.ended)
        assertEquals(setOf<String>(), manager.watching.value)
    }

    @Test
    fun aSecondEndWhileOneIsRunningIsIgnored() {
        core.endGate = CompletableDeferred()
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        manager.end("g")
        manager.end("g")
        assertEquals(listOf("g"), core.ended)
        assertEquals(setOf("g"), manager.ending.value)
        core.endGate!!.complete(Unit)
        assertEquals(setOf<String>(), manager.ending.value)
        assertEquals(1, notices.results.size)
    }

    @Test
    fun aFailedEndIsReportedAndRetriedWhenOnline() {
        core.endError = IllegalStateException("login expired")
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        val result = notices.results.single().third
        assertTrue(result.isFailure)
        assertEquals("login expired", result.exceptionOrNull()?.message)
        assertEquals(1, retries)
        assertEquals("the session is over all the same", setOf<String>(), manager.ending.value)
    }

    @Test
    fun anEndOfflineIsRetriedWhenOnline() {
        core.end = EndOutcome(offline = true)
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        assertEquals(1, retries)
    }

    @Test
    fun anUploadThatWorkedIsNotRetried() {
        core.end = EndOutcome(Notice(kind = NoticeKind.Uploaded))
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        assertEquals(0, retries)
    }

    @Test
    fun awaitEndedWaitsForTheUploadInProgress() = runBlocking {
        core.endGate = CompletableDeferred()
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        val waiting = async(Dispatchers.Unconfined) { manager.awaitEnded("g", timeoutMs = 5_000) }
        assertFalse(waiting.isCompleted)
        core.endGate!!.complete(Unit)
        waiting.await()
        assertTrue(waiting.isCompleted)
    }

    @Test
    fun awaitEndedGivesUpAfterTheTimeout() = runBlocking {
        core.endGate = CompletableDeferred() // never completes
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        manager.awaitEnded("g", timeoutMs = 50) // returns instead of hanging
        assertEquals(setOf("g"), manager.ending.value)
    }

    @Test
    fun awaitEndedReturnsAtOnceWhenNothingIsEnding() = runBlocking {
        manager.awaitEnded("g", timeoutMs = 5_000)
    }

    // A game that was left may come back

    private fun leave(): Long {
        manager.watch("g", "pkg.game", "Game")
        front.events += FrontEvent("pkg.game", 500)
        front.events += FrontEvent("other.app", 2_000)
        at(12_100)
        manager.check()
        assertEquals(listOf("g"), core.ended)
        return 12_100
    }

    @Test
    fun aGameThatComesBackIsOfferedToBeTrackedAgain() {
        leave()
        assertEquals("waiting for it: the service stays up", 0, host.idle)
        front.events += FrontEvent("pkg.game", 20_000)
        at(20_500)
        manager.check()
        assertEquals(listOf("g"), notices.offers)
        assertEquals("no longer waiting", 1, host.idle)
    }

    @Test
    fun aScreenFromBeforeTheEndDoesNotCountAsComingBack() {
        val endedAt = leave()
        front.events += FrontEvent("pkg.game", endedAt - 1)
        at(endedAt + 1_000)
        manager.check()
        assertEquals(emptyList<String>(), notices.offers)
    }

    @Test
    fun theWaitForAGameToComeBackExpires() {
        val endedAt = leave()
        at(endedAt + SessionManager.RECENT_MS + 1)
        manager.check()
        assertEquals(emptyList<String>(), notices.offers)
        assertEquals(1, host.idle)
        assertFalse(manager.hasWork)
    }

    @Test
    fun doneDoesNotWaitForTheGameToComeBack() {
        manager.watch("g", "pkg.game", "Game")
        manager.done("g")
        assertFalse(manager.hasWork)
        assertEquals(1, host.idle)
    }

    @Test
    fun watchingAgainCancelsTheOffer() {
        leave()
        manager.watch("g", "pkg.game", "Game")
        assertTrue("g" in notices.cancelled)
        assertFalse("not waiting for it any more", manager.snapshot().waiting.contains("Game"))
    }

    // Open sessions of a killed process

    private val game = GameView(id = "gid", name = "Game", device = DeviceConfig(exe = "pkg.game"))
    private val emulator = EmulatorView(id = "emu", name = "Eden", device = EmulatorDevice(exe = "pkg.eden/.Main"))

    @Test
    fun anOpenSessionIsPickedUpAndWatched() = runBlocking {
        core.open = listOf(OpenSession(key = "gid", game = "gid"))
        core.games = listOf(game)
        at(100_000)
        manager.recover(afterBoot = false)
        assertEquals(setOf("gid"), manager.watching.value)
        assertEquals(emptyList<String>(), core.ended)
        assertEquals(listOf("Game"), manager.snapshot().playing)
    }

    @Test
    fun aPickedUpEmulatorSessionIsTitledWithItsEmulator() = runBlocking {
        core.open = listOf(OpenSession(key = "gid.emu", game = "gid", emulator = "emu"))
        core.games = listOf(game)
        core.emulators = listOf(emulator)
        manager.recover(afterBoot = false)
        assertEquals(listOf("Game (Eden)"), manager.snapshot().playing)
    }

    @Test
    fun aPickedUpGameIsInFrontUntilAnotherAppIs() = runBlocking {
        core.open = listOf(OpenSession(key = "gid", game = "gid"))
        core.games = listOf(game)
        at(100_000)
        manager.recover(afterBoot = false)
        at(500_000)
        manager.check() // no event since: the game may still be running
        assertEquals(emptyList<String>(), core.ended)
        front.events += FrontEvent("other.app", 501_000)
        at(520_000)
        manager.check()
        assertEquals(listOf("gid"), core.ended)
    }

    @Test
    fun afterARebootOpenSessionsEndAtOnce() = runBlocking {
        core.open = listOf(OpenSession(key = "gid", game = "gid"))
        core.games = listOf(game)
        manager.recover(afterBoot = true)
        assertEquals(listOf("gid"), core.ended)
    }

    @Test
    fun anOpenSessionWithNoKnownAppEnds() = runBlocking {
        core.open = listOf(OpenSession(key = "gone", game = "gone"))
        manager.recover(afterBoot = false)
        assertEquals(listOf("gone"), core.ended)
        assertEquals("the key stands for the title", "gone", notices.results.single().first)
    }

    @Test
    fun recoverLeavesAnEndingSessionAlone() = runBlocking {
        core.endGate = CompletableDeferred()
        core.open = listOf(OpenSession(key = "a", game = "gid"))
        core.games = listOf(game)
        manager.watch("a", "pkg.game", "A")
        manager.done("a") // uploading
        manager.recover(afterBoot = true)
        assertEquals("ended once, not twice", listOf("a"), core.ended)
    }

    @Test
    fun recoverLeavesAWatchedSessionAlone() = runBlocking {
        core.open = listOf(OpenSession(key = "a", game = "gid"))
        core.games = listOf(game)
        manager.watch("a", "pkg.game", "A")
        manager.recover(afterBoot = true)
        assertEquals(emptyList<String>(), core.ended)
        assertEquals(setOf("a"), manager.watching.value)
    }

    // Back in Carryover

    @Test
    fun withoutUsageAccessComingBackToCarryoverEndsWatchedSessions() = runBlocking {
        front.usage = false
        manager.watch("g", "pkg.game", "Game")
        at(10_000)
        manager.appResumed()
        assertEquals(listOf("g"), core.ended)
    }

    @Test
    fun aSessionJustStartedSurvivesTheUserBeingStillInCarryover() = runBlocking {
        front.usage = false
        manager.watch("g", "pkg.game", "Game")
        at(1_000)
        manager.appResumed()
        assertEquals(emptyList<String>(), core.ended)
    }

    @Test
    fun withUsageAccessComingBackToCarryoverEndsNothing() = runBlocking {
        manager.watch("g", "pkg.game", "Game")
        at(100_000)
        manager.appResumed()
        assertEquals(emptyList<String>(), core.ended)
    }
}
