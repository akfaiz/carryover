package com.akfaiz.carryover.session

import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import com.akfaiz.carryover.core.AppLanguage
import com.akfaiz.carryover.core.hasUsageAccess
import com.akfaiz.carryover.core.Core
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * The foreground service that keeps the process alive while play sessions are
 * watched, ended or waited for, and polls for the front app. The rules are
 * [SessionManager]'s (when a session ends, what a failed upload does): this is
 * only the Android shell around it: the commands from [Sessions], the
 * notification and [BootReceiver] (a restarted service picks the open sessions
 * up again, see docs/android-architecture.md).
 *
 * "Left" is decided by the manager from [UsageFrontSource]: another app came
 * to the front and the game's didn't come back within the grace period (the
 * user's setting), or Done playing, or, without Usage access, coming back to
 * Carryover. Turning the screen off doesn't end a session.
 */
class SessionService : Service(), SessionHost {
    private val manager: SessionManager get() = SessionManager.shared
    private lateinit var notifier: SessionNotifier
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private var poll: Job? = null

    /** Set when the manager says nothing is left to do, until the next command. */
    @Volatile
    private var stopping = false

    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(AppLanguage.wrap(base))
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        SessionNotifier.createChannels(this)
        notifier = SessionNotifier(applicationContext)
        manager.attach(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        stopping = false
        startForegroundNow()
        if (Core.initError != null) {
            stopSelf()
            return START_NOT_STICKY
        }
        val m = manager
        when (intent?.action) {
            ACTION_WATCH -> m.watch(
                intent.getStringExtra(EXTRA_KEY).orEmpty(),
                intent.getStringExtra(EXTRA_PACKAGE).orEmpty(),
                intent.getStringExtra(EXTRA_TITLE).orEmpty(),
            )
            ACTION_DONE -> m.done(intent.getStringExtra(EXTRA_KEY))
            ACTION_APP_RESUMED -> scope.launch { m.appResumed() }
            ACTION_RECOVER -> scope.launch { m.recover(intent.getBooleanExtra(EXTRA_AFTER_BOOT, false)) }
            null -> scope.launch { m.recover(afterBoot = false) } // restarted after being killed
        }
        ensurePolling()
        changed()
        return START_STICKY
    }

    override fun onDestroy() {
        manager.detach(this)
        scope.cancel()
        super.onDestroy()
    }

    override fun changed() {
        if (!stopping) notifier.refresh(manager.snapshot(), hasUsageAccess(this))
    }

    override fun idle() {
        stopping = true
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun startForegroundNow() {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
        } else {
            0
        }
        val n = notifier.ongoing(manager.snapshot(), hasUsageAccess(this))
        ServiceCompat.startForeground(this, SessionNotifier.ONGOING_ID, n, type)
    }

    private fun ensurePolling() {
        if (poll?.isActive == true) return
        manager.startPolling()
        poll = scope.launch {
            while (isActive) {
                if (manager.hasWork) manager.check()
                delay(POLL_MS)
            }
        }
    }

    companion object {
        const val ACTION_WATCH = "com.akfaiz.carryover.session.WATCH"
        const val ACTION_DONE = "com.akfaiz.carryover.session.DONE"
        const val ACTION_APP_RESUMED = "com.akfaiz.carryover.session.APP_RESUMED"
        const val ACTION_RECOVER = "com.akfaiz.carryover.session.RECOVER"
        const val EXTRA_KEY = "key"
        const val EXTRA_PACKAGE = "package"
        const val EXTRA_TITLE = "title"
        const val EXTRA_AFTER_BOOT = "afterBoot"
        private const val POLL_MS = 2_000L

        private fun start(ctx: Context, intent: Intent) {
            ContextCompat.startForegroundService(ctx, intent.setClass(ctx, SessionService::class.java))
        }

        /** Watches the session of key, whose game runs in pkg. */
        fun watch(ctx: Context, key: String, pkg: String, title: String) = start(
            ctx,
            Intent(ACTION_WATCH).putExtra(EXTRA_KEY, key).putExtra(EXTRA_PACKAGE, pkg).putExtra(EXTRA_TITLE, title),
        )

        /** Ends the session of key now (key null: all). */
        fun done(ctx: Context, key: String?) = start(ctx, Intent(ACTION_DONE).apply { key?.let { putExtra(EXTRA_KEY, it) } })

        fun appResumed(ctx: Context) = start(ctx, Intent(ACTION_APP_RESUMED))

        fun recover(ctx: Context, afterBoot: Boolean) =
            start(ctx, Intent(ACTION_RECOVER).putExtra(EXTRA_AFTER_BOOT, afterBoot))
    }
}
