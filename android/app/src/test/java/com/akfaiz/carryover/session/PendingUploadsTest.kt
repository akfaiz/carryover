package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Notice
import com.akfaiz.carryover.core.NoticeKind
import com.akfaiz.carryover.core.OpenSession
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** The background retry of uploads that couldn't happen offline. */
class PendingUploadsTest {
    private val core = FakeCore()
    private val notices = FakeNotices()
    private var watching = emptySet<String>()

    private fun finish() = runBlocking { PendingUploads.finish(core, { watching }, notices) }

    private val uploaded = EndOutcome(Notice(kind = NoticeKind.Uploaded))
    private val session = OpenSession(key = "g", game = "gid")

    @Test
    fun nothingPendingIsSilent() {
        assertFalse(finish())
        assertEquals(emptyList<String>(), core.endedPending)
        assertEquals(0, notices.results.size)
    }

    @Test
    fun anUploadThatWorkedIsToldWithTheGamesName() {
        core.pending = listOf(session)
        core.games = listOf(GameView(id = "gid", name = "Game"))
        core.endPending["g"] = uploaded
        assertFalse(finish())
        val (title, key, result) = notices.results.single()
        assertEquals("Game", title)
        assertEquals("g", key)
        assertTrue(result.isSuccess)
    }

    @Test
    fun theEmulatorIsInTheTitle() {
        core.pending = listOf(OpenSession(key = "gid.e", game = "gid", emulator = "e"))
        core.games = listOf(GameView(id = "gid", name = "Game"))
        core.emulators = listOf(EmulatorView(id = "e", name = "Eden"))
        core.endPending["gid.e"] = uploaded
        finish()
        assertEquals("Game (Eden)", notices.results.single().first)
    }

    @Test
    fun stillOfflineAsksForAnotherTryAndStops() {
        core.pending = listOf(session, OpenSession(key = "h", game = "gid"))
        core.endPending["g"] = EndOutcome(offline = true)
        assertTrue(finish())
        assertEquals("the rest waits for the next try", listOf("g"), core.endedPending)
        assertEquals(0, notices.results.size)
    }

    @Test
    fun aRetryThatFoundNothingToDoIsSilent() {
        core.pending = listOf(session) // already finished, or played again: the core answers nothing
        assertFalse(finish())
        assertEquals(0, notices.results.size)
    }

    @Test
    fun aFailureIsToldAndTheNextOneStillRuns() {
        core.pending = listOf(session, OpenSession(key = "h", game = "gid"))
        core.endPendingErrors["g"] = IllegalStateException("login expired")
        core.endPending["h"] = uploaded
        assertFalse(finish())
        assertEquals(listOf("g", "h"), core.endedPending)
        assertEquals(listOf("g", "h"), notices.results.map { it.second })
        assertTrue(notices.results[0].third.isFailure)
        assertTrue(notices.results[1].third.isSuccess)
    }

    @Test
    fun aSessionPlayedAgainMeanwhileIsLeftAlone() {
        core.pending = listOf(session)
        watching = setOf("g")
        assertFalse(finish())
        assertEquals(emptyList<String>(), core.endedPending)
    }
}
