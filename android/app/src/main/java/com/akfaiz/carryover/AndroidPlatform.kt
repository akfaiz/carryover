package com.akfaiz.carryover

import android.app.Application
import android.net.Uri
import com.akfaiz.carryover.core.BugReport
import com.akfaiz.carryover.core.CoverImages
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.data.AppPlatform
import com.akfaiz.carryover.launch.LaunchActivity
import com.akfaiz.carryover.session.PlayOutcome
import com.akfaiz.carryover.session.SessionManager
import com.akfaiz.carryover.session.SessionService
import com.akfaiz.carryover.session.Sessions
import kotlinx.coroutines.flow.StateFlow
import java.io.File

/** [AppPlatform] on the real Android: the service, intents, shortcuts, the content resolver. */
class AndroidPlatform(private val app: Application) : AppPlatform {
    override val ending: StateFlow<Set<String>> get() = SessionManager.shared.ending

    override suspend fun play(key: String, native: Boolean, title: String, p: Profile, emulators: List<EmulatorView>): Result<PlayOutcome> =
        Sessions.launchable(app, p, emulators).map { Sessions.play(app, key, native, title, it.intent, it.pkg) }

    override fun rememberRecent(key: String, title: String, coverPath: String) = LaunchActivity.pushRecent(app, key, title, coverPath)

    override fun pinShortcut(key: String, title: String, coverPath: String): Boolean =
        LaunchActivity.pinShortcut(app, key, title, coverPath)

    override fun donePlaying(key: String) = SessionService.done(app, key)

    override fun appResumed() = SessionService.appResumed(app)

    override suspend fun prepareCover(uri: String): File = CoverImages.prepare(app, Uri.parse(uri))

    override suspend fun buildLogs(): File = BugReport.build(app)

    override suspend fun saveLogs(zip: File, target: String) {
        BugReport.save(app, zip, Uri.parse(target))
    }
}
