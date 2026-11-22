package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.PlanView
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.core.Summary
import com.akfaiz.carryover.core.SyncAction
import com.akfaiz.carryover.core.SyncAllOutcome
import com.akfaiz.carryover.core.SyncAllProgress
import com.akfaiz.carryover.core.SyncCore
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.core.SyncOutcome
import com.akfaiz.carryover.core.SyncResult
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

internal class FakeSyncCore : SyncCore {
    var outcome = SyncOutcome(SyncAction.None)
    var outcomeAll = SyncAllOutcome()
    var syncError: Exception? = null
    var planError: Exception? = null
    var plan = PlanView(SyncAction.Conflict, local = Summary(files = 3), remote = Summary(files = 5), hasRemote = true)

    /** When set, a sync waits for it: it is "running". */
    var gate: CompletableDeferred<Unit>? = null
    val synced = mutableListOf<Pair<String, Resolution?>>()
    val cancelled = mutableListOf<String>()
    var syncAllCalls = 0

    override suspend fun plan(key: String): PlanView {
        planError?.let { throw it }
        return plan
    }

    override suspend fun sync(key: String, resolution: Resolution?): SyncOutcome {
        synced += key to resolution
        gate?.await()
        syncError?.let { throw it }
        return outcome
    }

    override suspend fun syncAll(): SyncAllOutcome {
        syncAllCalls++
        gate?.await()
        syncError?.let { throw it }
        return outcomeAll
    }

    override fun cancel(key: String): Boolean {
        cancelled += key
        return true
    }
}

class SyncControllerTest {
    private val core = FakeSyncCore()
    private val sync = SyncController(core)

    private fun run(key: String = "k", resolution: Resolution? = null) =
        runBlocking { sync.sync(key, "Game", resolution) }

    @Test
    fun saysWhatAPlainSyncDid() {
        core.outcome = SyncOutcome(SyncAction.Push)
        assertEquals(SyncReport.Pushed("Game"), run())
        core.outcome = SyncOutcome(SyncAction.Pull)
        assertEquals(SyncReport.Pulled("Game"), run())
        core.outcome = SyncOutcome(SyncAction.None)
        assertEquals(SyncReport.UpToDate("Game"), run())
        core.outcome = SyncOutcome(offline = true)
        assertEquals(SyncReport.Offline("Game"), run())
        core.outcome = SyncOutcome(cancelled = true)
        assertEquals(SyncReport.Cancelled("Game"), run())
    }

    @Test
    fun offlineAndCancelledComeBeforeTheAction() {
        core.outcome = SyncOutcome(SyncAction.Push, offline = true)
        assertEquals(SyncReport.Offline("Game"), run())
    }

    @Test
    fun theKeyIsBusyOnlyWhileItSyncs() = runBlocking {
        core.gate = CompletableDeferred()
        val running = async(Dispatchers.Unconfined) { sync.sync("k", "Game") }
        assertTrue("k" in sync.state.value.syncing)
        core.gate!!.complete(Unit)
        running.await()
        assertEquals(emptyMap<String, SyncEvent?>(), sync.state.value.syncing)
    }

    @Test
    fun aSecondRequestForTheSameGameIsIgnored() = runBlocking {
        core.gate = CompletableDeferred()
        val first = async(Dispatchers.Unconfined) { sync.sync("k", "Game") }
        // Must return at once: a guard that fails would wait for the running sync.
        assertNull(withTimeout(2_000) { sync.sync("k", "Game") })
        assertEquals("only one reached the core", 1, core.synced.size)
        core.gate!!.complete(Unit)
        assertNotNull(first.await())
    }

    @Test
    fun otherGamesSyncAtTheSameTime() = runBlocking {
        core.gate = CompletableDeferred()
        val a = async(Dispatchers.Unconfined) { sync.sync("a", "A") }
        val b = async(Dispatchers.Unconfined) { sync.sync("b", "B") }
        assertEquals(setOf("a", "b"), sync.state.value.syncing.keys)
        core.gate!!.complete(Unit)
        a.await()
        b.await()
        Unit
    }

    @Test
    fun aConflictFetchesWhatEachSideHasAndWaitsForTheUser() {
        core.outcome = SyncOutcome(SyncAction.Conflict)
        assertEquals(SyncReport.NeedsDecision("Game"), run())
        val c = sync.state.value.conflict!!
        assertEquals("k", c.key)
        assertEquals("Game", c.name)
        assertEquals(3, c.plan.local.files)
        assertEquals(5, c.plan.remote.files)
        assertEquals("the sync itself is over", emptyMap<String, SyncEvent?>(), sync.state.value.syncing)
    }

    @Test
    fun takingTheConflictClearsItOnce() {
        core.outcome = SyncOutcome(SyncAction.Conflict)
        run()
        assertEquals("k", sync.takeConflict()?.key)
        assertNull(sync.takeConflict())
        assertNull(sync.state.value.conflict)
    }

    @Test
    fun theUsersDecisionIsPassedToTheCore() {
        run(resolution = Resolution.Cloud)
        assertEquals(listOf("k" to Resolution.Cloud), core.synced)
    }

    @Test
    fun startingASyncDropsAnOldConflict() {
        core.outcome = SyncOutcome(SyncAction.Conflict)
        run("a")
        core.outcome = SyncOutcome(SyncAction.None)
        run("b")
        assertNull(sync.state.value.conflict)
    }

    @Test
    fun anErrorIsReportedWithItsCauseAndTheKeyIsFreed() {
        core.syncError = IllegalStateException("disk full")
        val r = run() as SyncReport.Failed
        assertEquals("Game", r.name)
        assertEquals("disk full", r.cause.message)
        assertEquals(emptyMap<String, SyncEvent?>(), sync.state.value.syncing)
    }

    @Test
    fun aConflictWhosePlanCantBeReadIsAFailure() {
        core.outcome = SyncOutcome(SyncAction.Conflict)
        core.planError = IllegalStateException("cloud gone")
        val r = run() as SyncReport.Failed
        assertEquals("cloud gone", r.cause.message)
        assertNull(sync.state.value.conflict)
        assertEquals(emptyMap<String, SyncEvent?>(), sync.state.value.syncing)
    }

    @Test
    fun cancelGoesToTheCore() {
        sync.cancel("k")
        assertEquals(listOf("k"), core.cancelled)
    }

    // Progress

    @Test
    fun progressUpdatesOnlyTheGamesBeingSynced() = runBlocking {
        core.gate = CompletableDeferred()
        val running = async(Dispatchers.Unconfined) { sync.sync("k", "Game") }
        sync.onSync(SyncEvent(gameId = "k", phase = "upload", bytes = 5, totalBytes = 10))
        sync.onSync(SyncEvent(gameId = "other", phase = "upload"))
        assertEquals(5L, sync.state.value.syncing["k"]?.bytes)
        assertEquals(setOf("k"), sync.state.value.syncing.keys)
        core.gate!!.complete(Unit)
        running.await()
        Unit
    }

    @Test
    fun progressOfAGameNotSyncingHereIsIgnored() {
        sync.onSync(SyncEvent(gameId = "k", phase = "upload"))
        assertEquals(SyncState(), sync.state.value)
    }

    // Sync all

    @Test
    fun syncAllReportsTheResults() {
        core.outcomeAll = SyncAllOutcome(
            listOf(
                SyncResult(gameId = "a", name = "A", action = SyncAction.Conflict),
                SyncResult(gameId = "b", name = "B", error = "denied"),
                SyncResult(gameId = "c", name = "C", action = SyncAction.Push),
            ),
        )
        val r = runBlocking { sync.syncAll() } as SyncReport.AllDone
        assertEquals(listOf("B"), r.failed.map { it.name })
        assertEquals(1, r.conflicts)
        assertEquals(3, r.results.size)
    }

    @Test
    fun syncAllOfflineOrCancelled() {
        core.outcomeAll = SyncAllOutcome(offline = true)
        assertEquals(SyncReport.AllOffline, runBlocking { sync.syncAll() })
        core.outcomeAll = SyncAllOutcome(cancelled = true)
        assertEquals(SyncReport.AllCancelled, runBlocking { sync.syncAll() })
    }

    @Test
    fun syncAllErrorIsReportedAndNothingStaysBusy() {
        core.syncError = IllegalStateException("no network stack")
        val r = runBlocking { sync.syncAll() } as SyncReport.AllFailed
        assertEquals("no network stack", r.cause.message)
        assertNull(sync.state.value.syncAll)
    }

    @Test
    fun syncAllIsShownWhileItRunsAndOnceAtATime() = runBlocking {
        core.gate = CompletableDeferred()
        val running = async(Dispatchers.Unconfined) { sync.syncAll() }
        assertEquals(SyncAllProgress(), sync.state.value.syncAll)
        assertNull(withTimeout(2_000) { sync.syncAll() })
        assertEquals(1, core.syncAllCalls)
        core.gate!!.complete(Unit)
        running.await()
        assertNull(sync.state.value.syncAll)
        assertNull(sync.state.value.syncAllEvent)
    }

    @Test
    fun syncAllProgressKeepsTheLastGamesName() = runBlocking {
        core.gate = CompletableDeferred()
        val running = async(Dispatchers.Unconfined) { sync.syncAll() }
        sync.onSyncAll(SyncAllProgress(done = 1, total = 2, name = "Zelda", key = "z"))
        assertEquals("Zelda", sync.state.value.syncAll?.name)
        // The final event (all done) names no game.
        sync.onSyncAll(SyncAllProgress(done = 2, total = 2))
        assertEquals("Zelda", sync.state.value.syncAll?.name)
        assertEquals(2, sync.state.value.syncAll?.done)
        core.gate!!.complete(Unit)
        running.await()
        Unit
    }

    @Test
    fun syncAllProgressAfterItEndedIsIgnored() {
        sync.onSyncAll(SyncAllProgress(done = 1, total = 2, name = "Late"))
        assertNull(sync.state.value.syncAll)
    }

    @Test
    fun theTransferOfTheGameSyncAllIsOnIsTracked() = runBlocking {
        core.gate = CompletableDeferred()
        val running = async(Dispatchers.Unconfined) { sync.syncAll() }
        sync.onSyncAll(SyncAllProgress(done = 0, total = 2, name = "Zelda", key = "z"))
        sync.onSync(SyncEvent(gameId = "z", bytes = 7, totalBytes = 9))
        sync.onSync(SyncEvent(gameId = "other", bytes = 1))
        assertEquals(7L, sync.state.value.syncAllEvent?.bytes)
        // The next game starts: the old one's transfer is not shown for it.
        sync.onSyncAll(SyncAllProgress(done = 1, total = 2, name = "Mario", key = "m"))
        assertNull(sync.state.value.syncAllEvent)
        core.gate!!.complete(Unit)
        running.await()
        Unit
    }
}
