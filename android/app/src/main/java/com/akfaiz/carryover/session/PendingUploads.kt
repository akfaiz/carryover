package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.SessionCore
import kotlinx.coroutines.CancellationException

/**
 * Finishes the uploads of sessions that ended without a network (see
 * [com.akfaiz.carryover.work.SyncWorker]). It never ends a session that is
 * being played: the core only finishes sessions already marked as ended
 * ([SessionCore.endPendingSession]), and ones watched here are skipped.
 */
object PendingUploads {
    /**
     * Returns true when there is still no network, so the work should be tried
     * again later. Tells the user only when something happened: a retry that
     * finds nothing to do is silent.
     */
    suspend fun finish(
        core: SessionCore,
        watching: () -> Set<String>,
        notices: SessionNotices,
        log: (String) -> Unit = {},
    ): Boolean {
        val pending = runCatching { core.pendingSessions() }.getOrDefault(emptyList())
        if (pending.isEmpty()) return false
        val games = runCatching { core.games() }.getOrDefault(emptyList())
        val emulators = runCatching { core.emulators() }.getOrDefault(emptyList())
        for (s in pending) {
            if (s.key in watching()) continue // played again meanwhile
            val result: Result<EndOutcome> = try {
                Result.success(core.endPendingSession(s.key))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                log("end pending ${s.key}: ${e.message}")
                Result.failure(e)
            }
            val out = result.getOrNull()
            if (out?.offline == true) return true // still no network
            if (result.isFailure || out?.notice != null) notices.result(s.title(games, emulators), s.key, result)
        }
        return false
    }
}
