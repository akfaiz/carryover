package com.akfaiz.carryover.launch

import android.app.Application
import android.content.Intent
import android.net.Uri
import android.util.Log
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.session.PlayOutcome
import com.akfaiz.carryover.session.Sessions

/** [LaunchPlatform] on the real Android, for the [intent] that started [LaunchActivity]. */
class AndroidLaunchPlatform(private val app: Application, private val intent: Intent) : LaunchPlatform {
    override val request = LaunchRequest(
        key = intent.getStringExtra(Forward.EXTRA_KEY),
        track = intent.getBooleanExtra(Forward.EXTRA_TRACK, false),
        target = intent.getStringExtra(Forward.EXTRA_TARGET),
        data = intent.dataString,
        extras = stringExtras(intent),
    )

    override val ownPackage: String get() = app.packageName

    /** What starts the target: set by [prepareForward]. */
    private var forward: Intent? = null

    override fun appLabel(pkg: String): String = runCatching {
        val pm = app.packageManager
        pm.getApplicationLabel(pm.getApplicationInfo(pkg, 0)).toString()
    }.getOrDefault(pkg)

    override fun prepareForward(target: String): Boolean {
        forward = forwardIntent(intent, target)
        return forward != null
    }

    override suspend fun play(cand: Forward.Candidate, emulators: List<EmulatorView>): Result<PlayOutcome> =
        Sessions.launchable(app, cand.profile, emulators).map {
            Sessions.play(app, cand.profile.key, cand.profile.emulatorId == null, cand.title, it.intent, it.pkg)
        }

    override suspend fun playForward(cand: Forward.Candidate, pkg: String): PlayOutcome =
        Sessions.play(app, cand.profile.key, cand.profile.emulatorId == null, cand.title, checkNotNull(forward), pkg)

    override suspend fun track(cand: Forward.Candidate, pkg: String): PlayOutcome =
        Sessions.track(app, cand.profile.key, cand.profile.emulatorId == null, cand.title, pkg)

    override fun startWithoutSync(): String? = try {
        app.startActivity(checkNotNull(forward))
        null
    } catch (e: Exception) {
        e.message ?: e.toString()
    }

    override fun rememberRecent(cand: Forward.Candidate) =
        LaunchActivity.pushRecent(app, cand.profile.key, cand.title, cand.game.coverPath)

    override fun log(what: String, e: Throwable) {
        Log.w(TAG, what, e)
    }

    /**
     * The intent to start target with: a copy of the launch (action, data,
     * type, categories, extras but Carryover's, clip data), sent to target in
     * a task of its own. URI read grants are passed on only for URIs the
     * caller granted with the launch, and never for Carryover's own save
     * folders. Null when target doesn't resolve to an exported activity of
     * that app.
     */
    private fun forwardIntent(incoming: Intent, target: String): Intent? {
        val pkg = Forward.packageOf(target)
        val activity = target.substringAfter('/', "").trim()
        val out = Intent(incoming)
        out.component = null
        if (activity.isNotEmpty()) out.setClassName(pkg, if (activity.startsWith(".")) pkg + activity else activity) else out.setPackage(pkg)
        incoming.extras?.keySet()?.filter { it.startsWith(Forward.EXTRA_PREFIX) }?.forEach(out::removeExtra)

        val grantFlags = Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
            Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION or Intent.FLAG_GRANT_PREFIX_URI_PERMISSION
        val callerGranted = incoming.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0
        out.flags = (incoming.flags and grantFlags.inv()) or Intent.FLAG_ACTIVITY_NEW_TASK
        val uris = buildList<Uri> {
            incoming.data?.let(::add)
            incoming.clipData?.let { clip -> for (i in 0 until clip.itemCount) clip.getItemAt(i).uri?.let(::add) }
        }.map { it.toString() }.filter { it.startsWith("content://") }
        if (uris.isNotEmpty()) {
            val persisted = app.contentResolver.persistedUriPermissions.map { it.uri.toString() }
            if (uris.all { Forward.mayGrant(it, callerGranted, persisted) }) {
                out.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            } else {
                out.clipData = null
            }
        }
        val info = app.packageManager.resolveActivity(out, 0)?.activityInfo ?: return null
        if (!info.exported || info.packageName != pkg) return null
        return out
    }

    private companion object {
        const val TAG = "CarryoverLaunch"

        fun stringExtras(intent: Intent): Map<String, String> {
            val extras = intent.extras ?: return emptyMap()
            return extras.keySet().mapNotNull { k ->
                @Suppress("DEPRECATION")
                (extras.get(k) as? String)?.let { k to it }
            }.toMap()
        }
    }
}
