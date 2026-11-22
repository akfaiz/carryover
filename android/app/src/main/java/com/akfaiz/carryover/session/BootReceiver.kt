package com.akfaiz.carryover.session

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.SystemClock
import android.util.Log
import com.akfaiz.carryover.core.Core
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/**
 * After a reboot no game runs: sessions left open (the phone was switched off
 * while playing) are ended, which uploads their saves when online.
 *
 * Android 15 also sends BOOT_COMPLETED when an app that was force-stopped
 * starts again, long after the device booted. The game may still run then,
 * so such sessions are picked up and watched instead of ended at once.
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED || Core.initError != null) return
        val pending = goAsync()
        val app = context.applicationContext
        val booted = SystemClock.elapsedRealtime() < JUST_BOOTED_MS
        CoroutineScope(Dispatchers.IO).launch {
            try {
                if (Core.openSessions().isNotEmpty()) {
                    Log.i("CarryoverSession", if (booted) "booted: ending open sessions" else "restarted: picking up open sessions")
                    SessionService.recover(app, afterBoot = booted)
                }
            } finally {
                pending.finish()
            }
        }
    }

    private companion object {
        const val JUST_BOOTED_MS = 10 * 60_000L
    }
}
