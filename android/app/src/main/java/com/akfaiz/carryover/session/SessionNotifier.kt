package com.akfaiz.carryover.session

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import com.akfaiz.carryover.MainActivity
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.NoticeKind
import com.akfaiz.carryover.launch.LaunchActivity

/**
 * Every notification of the play sessions: the ongoing one of the foreground
 * service (what is watched, with Done playing), how an end went, and the offer
 * to track a game that came back. [ctx] is the application's, so the texts
 * follow the app's language.
 */
class SessionNotifier(private val ctx: Context) : SessionNotices {
    private val nm: NotificationManager get() = ctx.getSystemService(NotificationManager::class.java)

    /** The ongoing notification of the foreground service, for what the manager is doing now. */
    fun ongoing(s: SessionManager.Snapshot, usageAccess: Boolean): Notification {
        val text = when {
            s.playing.isEmpty() && s.uploading -> ctx.getString(R.string.notif_uploading)
            s.playing.isEmpty() && s.waiting.isNotEmpty() -> ctx.getString(R.string.notif_waiting, s.waiting.joinToString(", "))
            s.playing.isEmpty() -> ctx.getString(R.string.notif_checking)
            usageAccess -> ctx.getString(R.string.notif_auto)
            else -> ctx.getString(R.string.notif_manual)
        }
        val b = NotificationCompat.Builder(ctx, CHANNEL_PLAYING)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(
                if (s.playing.isEmpty()) {
                    ctx.getString(R.string.app_name)
                } else {
                    ctx.getString(R.string.notif_playing, s.playing.joinToString(", "))
                },
            )
            .setContentText(text)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            // Its own group: bundled with result notifications, its Done playing button would be hidden.
            .setGroup(CHANNEL_PLAYING)
            .setContentIntent(openApp())
        if (s.playing.isNotEmpty()) {
            b.addAction(
                0,
                ctx.getString(R.string.action_done_playing),
                PendingIntent.getService(
                    ctx,
                    1,
                    Intent(ctx, SessionService::class.java).setAction(SessionService.ACTION_DONE),
                    PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
                ),
            )
        }
        return b.build()
    }

    fun refresh(s: SessionManager.Snapshot, usageAccess: Boolean) {
        runCatching { nm.notify(ONGOING_ID, ongoing(s, usageAccess)) }
    }

    /** Tells the user how ending the session of [key] went (a notification of the results channel). */
    override fun result(title: String, key: String, result: Result<EndOutcome>) {
        val out = result.getOrNull()
        val n = out?.notice
        val (head, body) = when {
            result.isFailure -> ctx.getString(R.string.result_failed, title) to (result.exceptionOrNull()?.message ?: "")
            out?.offline == true -> ctx.getString(R.string.result_offline, title) to ctx.getString(R.string.result_offline_text)
            n?.kind == NoticeKind.Uploaded -> ctx.getString(R.string.result_uploaded, title) to ctx.getString(R.string.result_uploaded_text)
            n?.kind == NoticeKind.Conflict -> ctx.getString(R.string.result_conflict, title) to ctx.getString(R.string.result_conflict_text)
            n?.kind == NoticeKind.Failed -> ctx.getString(R.string.result_failed, title) to n.err
            else -> ctx.getString(R.string.result_none, title) to ctx.getString(R.string.result_none_text)
        }
        val notif = NotificationCompat.Builder(ctx, CHANNEL_RESULTS)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(head)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setAutoCancel(true)
            .setGroup(CHANNEL_RESULTS)
            .setContentIntent(openApp())
            .build()
        runCatching { nm.notify(key.hashCode(), notif) }
    }

    override fun offerTracking(key: String, title: String) {
        val intent = PendingIntent.getActivity(
            ctx,
            returnedId(key),
            LaunchActivity.trackIntent(ctx, key),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val n = NotificationCompat.Builder(ctx, CHANNEL_RESULTS)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(ctx.getString(R.string.result_back, title))
            .setContentText(ctx.getString(R.string.result_back_text))
            .setStyle(NotificationCompat.BigTextStyle().bigText(ctx.getString(R.string.result_back_text)))
            .setAutoCancel(true)
            .setContentIntent(intent)
            .build()
        runCatching { nm.notify(returnedId(key), n) }
    }

    override fun cancelOffer(key: String) {
        nm.cancel(returnedId(key))
    }

    private fun openApp(): PendingIntent = PendingIntent.getActivity(
        ctx,
        0,
        Intent(ctx, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    companion object {
        const val CHANNEL_PLAYING = "session"
        const val CHANNEL_RESULTS = "results"
        const val ONGOING_ID = 1

        private fun returnedId(key: String) = ("back:$key").hashCode()

        fun createChannels(ctx: Context) {
            val nm = ctx.getSystemService(NotificationManager::class.java)
            nm.createNotificationChannel(
                // Default importance keeps it out of the collapsed "Silent" section, where its
                // Done playing button is hidden; it makes no sound.
                NotificationChannel(CHANNEL_PLAYING, ctx.getString(R.string.channel_playing), NotificationManager.IMPORTANCE_DEFAULT).apply {
                    description = ctx.getString(R.string.channel_playing_desc)
                    setSound(null, null)
                    enableVibration(false)
                },
            )
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_RESULTS, ctx.getString(R.string.channel_results), NotificationManager.IMPORTANCE_DEFAULT).apply {
                    description = ctx.getString(R.string.channel_results_desc)
                },
            )
        }
    }
}
