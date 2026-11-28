package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.BeginOutcome
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** Starting a play session: what runs, in which order, against a fake core. */
class PlayFlowTest {
    private val core = FakeCore()
    private val manager = SessionManager(
        core = core,
        notices = FakeNotices(),
        front = FakeFront(),
        scope = CoroutineScope(Dispatchers.Unconfined),
        graceMs = { 10_000 },
        retryUploads = {},
    )
    private val events = mutableListOf<String>()
    private val flow = PlayFlow(
        core = core,
        manager = manager,
        watch = { key, pkg, title -> events += "watch $key $pkg $title" },
        stopWatching = { key -> events += "stop $key" },
        startFailed = { title, cause -> "$title: ${cause.message}" },
    )

    private val noBegin = emptyList<Pair<String, String>>()

    @Test
    fun syncsThenWatchesThenStarts() = runBlocking {
        val out = flow.play("k", native = true, title = "Game", pkg = "pkg.game") {
            events += "launch"
            null
        }
        assertEquals(PlayOutcome.Started(false), out)
        assertEquals(listOf("watch k pkg.game Game", "launch"), events)
        assertEquals(listOf("k" to "native"), core.begun)
    }

    @Test
    fun anEmulatorProfileIsNotNative() = runBlocking {
        flow.play("g.emu", native = false, title = "Game (Eden)", pkg = "pkg.eden") { null }
        assertEquals(listOf("g.emu" to ""), core.begun)
    }

    @Test
    fun startsOfflineWhenThereIsNoNetwork() = runBlocking {
        core.begin = BeginOutcome(synced = true, preOffline = true)
        val out = flow.play("k", true, "Game", "pkg.game") { null }
        assertEquals(PlayOutcome.Started(offline = true), out)
    }

    @Test
    fun aCancelledSyncStartsNothing() = runBlocking {
        core.begin = BeginOutcome(cancelled = true)
        var launched = false
        val out = flow.play("k", true, "Game", "pkg.game") {
            launched = true
            null
        }
        assertEquals(PlayOutcome.Cancelled, out)
        assertFalse(launched)
        assertEquals(emptyList<String>(), events)
    }

    @Test
    fun aSessionThatWasNotTakenIsNotWatched() = runBlocking {
        core.begin = BeginOutcome(synced = false)
        val out = flow.play("k", true, "Game", "pkg.game") { null }
        assertEquals(PlayOutcome.Started(false), out)
        assertEquals(emptyList<String>(), events)
    }

    @Test
    fun aFailedStartReleasesTheSession() = runBlocking {
        val out = flow.play("k", true, "Game", "pkg.game") { "not installed" }
        assertEquals(PlayOutcome.Failed("not installed"), out)
        assertEquals(listOf("watch k pkg.game Game", "stop k"), events)
    }

    @Test
    fun aFailedSyncStartNamesTheGame() = runBlocking {
        core.beginError = IllegalStateException("no cloud")
        var launched = false
        val out = flow.play("k", true, "Game", "pkg.game") {
            launched = true
            null
        }
        assertEquals(PlayOutcome.Failed("Game: no cloud"), out)
        assertFalse(launched)
    }

    @Test
    fun aSessionStillOpenJustGoesOn() = runBlocking {
        core.openKeys += "k"
        var launched = false
        val out = flow.play("k", true, "Game", "pkg.game") {
            launched = true
            null
        }
        assertEquals(PlayOutcome.Started(false), out)
        assertTrue(launched)
        assertEquals("no new sync, no new lock", noBegin, core.begun)
        assertEquals(listOf("watch k pkg.game Game"), events)
    }

    @Test
    fun theUploadInProgressIsWaitedForFirst() = runBlocking {
        core.endGate = CompletableDeferred()
        manager.watch("k", "pkg.game", "Game")
        manager.done("k") // uploading

        val playing = async(Dispatchers.Unconfined) { flow.play("k", true, "Game", "pkg.game") { null } }
        assertFalse(playing.isCompleted)
        assertEquals("no lock taken while the old session uploads", noBegin, core.begun)

        core.endGate!!.complete(Unit)
        assertEquals(PlayOutcome.Started(false), playing.await())
        assertEquals(listOf("k" to "native"), core.begun)
    }

    @Test
    fun trackSyncsAndWatchesWithoutStartingAnything() = runBlocking {
        val out = flow.track("k", native = true, title = "Game", pkg = "pkg.game")
        assertEquals(PlayOutcome.Started(false), out)
        assertEquals(listOf("watch k pkg.game Game"), events)
    }

    @Test
    fun trackOfASessionStillOpenOnlyWatches() = runBlocking {
        core.openKeys += "k"
        flow.track("k", true, "Game", "pkg.game")
        assertEquals(noBegin, core.begun)
        assertEquals(listOf("watch k pkg.game Game"), events)
    }

    @Test
    fun trackCancelledWatchesNothing() = runBlocking {
        core.begin = BeginOutcome(cancelled = true)
        assertEquals(PlayOutcome.Cancelled, flow.track("k", true, "Game", "pkg.game"))
        assertEquals(emptyList<String>(), events)
    }
}
