package com.akfaiz.carryover

import android.app.Application
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
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

    override fun canInstallPackages(): Boolean = app.packageManager.canRequestPackageInstalls()

    override fun openInstallPermission(): Boolean = open(
        Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${app.packageName}")),
    )

    override fun installApk(path: String): Boolean {
        // Shared through the FileProvider (res/xml/file_paths.xml); only the installer is granted the read.
        val uri = FileProvider.getUriForFile(app, "${app.packageName}.files", File(path))
        return open(
            Intent(Intent.ACTION_VIEW)
                .setDataAndType(uri, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION),
        )
    }

    private fun open(intent: Intent): Boolean = try {
        app.startActivity(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        true
    } catch (e: ActivityNotFoundException) {
        false
    }

    override suspend fun buildLogs(): File = BugReport.build(app)

    override suspend fun saveLogs(zip: File, target: String) {
        BugReport.save(app, zip, Uri.parse(target))
    }
}
