package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.PlanView
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.core.SyncAction
import com.akfaiz.carryover.core.SyncAllProgress
import com.akfaiz.carryover.core.SyncCore
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.core.SyncResult
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.getAndUpdate
import kotlinx.coroutines.flow.update

/** Both sides changed: the user picks which saves to keep. */
data class Conflict(val key: String, val name: String, val plan: PlanView)

/** What is syncing now, as the pages show it. */
data class SyncState(
    /** Profile keys being synced, with their latest progress. */
    val syncing: Map<String, SyncEvent?> = emptyMap(),
    val syncAll: SyncAllProgress? = null,
    /** The latest progress of the game Sync all is on (bytes of its transfer). */
    val syncAllEvent: SyncEvent? = null,
    /** A conflict waiting for the user's decision. */
    val conflict: Conflict? = null,
)

/** How a sync went: the controller says what happened, the view model says it in words. */
sealed interface SyncReport {
    data class Offline(val name: String) : SyncReport
    data class Cancelled(val name: String) : SyncReport
    data class Pushed(val name: String) : SyncReport
    data class Pulled(val name: String) : SyncReport
    data class UpToDate(val name: String) : SyncReport

    /** Both sides changed: [SyncState.conflict] holds it. */
    data class NeedsDecision(val name: String) : SyncReport
    data class Failed(val name: String, val cause: Exception) : SyncReport

    data object AllOffline : SyncReport
    data object AllCancelled : SyncReport
    data class AllFailed(val cause: Exception) : SyncReport

    data class AllDone(val results: List<SyncResult>) : SyncReport {
        val failed: List<SyncResult> get() = results.filter { it.error.isNotEmpty() }
        val conflicts: Int get() = results.count { it.action == SyncAction.Conflict }
    }
}

/**
 * Syncing games, one or all: which are running (a second request for the same
 * game, or for Sync all, is ignored), how far they are, a conflict to decide,
 * and how each sync went. No Android classes: unit tested with a fake
 * [SyncCore]. The functions suspend for the sync's duration; the caller
 * launches them and tells the user the [SyncReport].
 */
class SyncController(private val core: SyncCore) {
    private val _state = MutableStateFlow(SyncState())
    val state: StateFlow<SyncState> = _state

    /**
     * Syncs profile [key] (named [name] for the user); [resolution] decides a
     * conflict. Null when it is already being synced.
     */
    suspend fun sync(key: String, name: String, resolution: Resolution? = null): SyncReport? {
        val before = _state.getAndUpdate {
            if (key in it.syncing) it else it.copy(syncing = it.syncing + (key to null), conflict = null)
        }
        if (key in before.syncing) return null
        return try {
            val out = core.sync(key, resolution)
            when {
                out.offline -> SyncReport.Offline(name)
                out.cancelled -> SyncReport.Cancelled(name)
                out.action == SyncAction.Conflict -> {
                    val plan = core.plan(key)
                    _state.update { it.copy(conflict = Conflict(key, name, plan)) }
                    SyncReport.NeedsDecision(name)
                }
                out.action == SyncAction.Push -> SyncReport.Pushed(name)
                out.action == SyncAction.Pull -> SyncReport.Pulled(name)
                else -> SyncReport.UpToDate(name)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            SyncReport.Failed(name, e)
        } finally {
            _state.update { it.copy(syncing = it.syncing - key) }
        }
    }

    /** Syncs every game. Null when Sync all is already running. */
    suspend fun syncAll(): SyncReport? {
        val before = _state.getAndUpdate { if (it.syncAll != null) it else it.copy(syncAll = SyncAllProgress()) }
        if (before.syncAll != null) return null
        return try {
            val out = core.syncAll()
            when {
                out.offline -> SyncReport.AllOffline
                out.cancelled -> SyncReport.AllCancelled
                else -> SyncReport.AllDone(out.results)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            SyncReport.AllFailed(e)
        } finally {
            _state.update { it.copy(syncAll = null, syncAllEvent = null) }
        }
    }

    /** Stops the running sync of [key]. */
    fun cancel(key: String) {
        core.cancel(key)
    }

    /** The conflict the user is deciding, which is then no longer pending (null: none). */
    fun takeConflict(): Conflict? = _state.getAndUpdate { it.copy(conflict = null) }.conflict

    /** Progress of a sync (the core reports it as events, from any thread). */
    fun onSync(event: SyncEvent) {
        _state.update { s ->
            var next = s
            // The game Sync all is on: how far its transfer is.
            if (s.syncAll != null && s.syncAll.key.isNotEmpty() && s.syncAll.key == event.gameId) {
                next = next.copy(syncAllEvent = event)
            }
            // Syncs started here (keys may be emulator keys).
            if (event.gameId in s.syncing) next = next.copy(syncing = next.syncing + (event.gameId to event))
            next
        }
    }

    /** Progress of Sync all, as a whole. */
    fun onSyncAll(progress: SyncAllProgress) {
        _state.update { s ->
            // While Sync all runs (syncAll() set it), keep it shown until it returns.
            val current = s.syncAll ?: return@update s
            // The last event (all done) names no game: keep the last one's name.
            val p = if (progress.name.isEmpty() && progress.total > 0) progress.copy(name = current.name) else progress
            s.copy(syncAll = p, syncAllEvent = s.syncAllEvent.takeIf { it?.gameId == progress.key })
        }
    }
}
