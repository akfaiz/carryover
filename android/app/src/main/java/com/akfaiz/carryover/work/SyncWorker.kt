package com.akfaiz.carryover.work

import android.content.Context
import android.util.Log
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.akfaiz.carryover.core.Core
import com.akfaiz.carryover.session.PendingUploads
import com.akfaiz.carryover.session.SessionManager
import com.akfaiz.carryover.session.SessionNotifier

/**
 * Background work (see [BackgroundSync]): finishes the uploads of sessions
 * that ended without a network ([PendingUploads]), and, for the periodic run,
 * syncs all games.
 */
class SyncWorker(ctx: Context, params: WorkerParameters) : CoroutineWorker(ctx, params) {
    override suspend fun doWork(): Result {
        if (Core.initError != null) return Result.success()
        val periodic = inputData.getBoolean(BackgroundSync.KEY_PERIODIC, false)

        val stillOffline = PendingUploads.finish(
            Core,
            watching = { SessionManager.shared.watching.value },
            notices = SessionNotifier(applicationContext),
            log = { Log.w(TAG, it) },
        )
        if (stillOffline) return Result.retry()

        if (periodic) {
            // Failures are the next period's business: sync errors are flagged on each game.
            runCatching { Core.syncAll() }.exceptionOrNull()?.let { Log.w(TAG, "periodic sync", it) }
        }
        return Result.success()
    }

    private companion object {
        const val TAG = "CarryoverWork"
    }
}
