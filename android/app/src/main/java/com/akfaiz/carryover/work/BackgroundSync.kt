package com.akfaiz.carryover.work

import android.content.Context
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.workDataOf
import com.akfaiz.carryover.core.AppPrefs
import java.util.concurrent.TimeUnit

/**
 * Work that runs without the app in front (WorkManager, so it survives the
 * process and reboots): finishing uploads that couldn't happen offline, and,
 * if the user turned it on, syncing every game now and then.
 */
object BackgroundSync {
    private const val RETRY = "carryover-retry-uploads"
    private const val PERIODIC = "carryover-periodic-sync"
    internal const val KEY_PERIODIC = "periodic"
    private const val PERIOD_HOURS = 6L

    /**
     * Uploads the saves of sessions that ended offline once the device has a
     * network. Asking again while one is waiting changes nothing.
     */
    fun retryUploads(ctx: Context) {
        val request = OneTimeWorkRequestBuilder<SyncWorker>()
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 1, TimeUnit.MINUTES)
            .build()
        WorkManager.getInstance(ctx).enqueueUniqueWork(RETRY, ExistingWorkPolicy.KEEP, request)
    }

    /**
     * Applies the "Sync in the background" setting: every few hours, on an
     * unmetered network, while charging and idle (so no game is being played).
     */
    fun schedule(ctx: Context) {
        val wm = WorkManager.getInstance(ctx)
        if (!AppPrefs.backgroundSync(ctx)) {
            wm.cancelUniqueWork(PERIODIC)
            return
        }
        val request = PeriodicWorkRequestBuilder<SyncWorker>(PERIOD_HOURS, TimeUnit.HOURS)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.UNMETERED)
                    .setRequiresCharging(true)
                    .setRequiresDeviceIdle(true)
                    .build(),
            )
            .setInputData(workDataOf(KEY_PERIODIC to true))
            .build()
        wm.enqueueUniquePeriodicWork(PERIODIC, ExistingPeriodicWorkPolicy.KEEP, request)
    }
}
