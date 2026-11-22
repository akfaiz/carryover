package com.akfaiz.carryover.core

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Process
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

/**
 * The logs for a bug report, as one zip: the core's log files (what the
 * desktop asks users to attach), Android's log of this run of the app (the
 * Kotlin side logs only there: session watcher, launches, SAF folders) and a
 * report.txt with the versions, the device and the permissions. The log lives
 * in the app's private storage, so sharing or saving this zip is the only way
 * users can reach it. Cloud account settings (rclone.conf) are never included.
 */
object BugReport {
    /** Where the zips are made; shared through FileProvider (res/xml/file_paths.xml). */
    private const val DIR = "reports"

    /** Android's log of this process is cut to its last part beyond this size. */
    private const val MAX_LOGCAT = 4 shl 20

    /** Builds the zip in the cache (replacing older ones) and returns it. */
    suspend fun build(ctx: Context): File = withContext(Dispatchers.IO) {
        val dir = File(ctx.cacheDir, DIR).apply { mkdirs() }
        dir.listFiles()?.forEach { it.delete() }
        val stamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyyMMdd-HHmmss"))
        val zip = File(dir, "carryover-logs-$stamp.zip")
        ZipOutputStream(zip.outputStream().buffered()).use { out ->
            fun add(name: String, bytes: ByteArray) {
                out.putNextEntry(ZipEntry(name))
                out.write(bytes)
                out.closeEntry()
            }
            add("report.txt", report(ctx).toByteArray())
            val logs = Core.logFile.takeIf { it.isNotEmpty() }?.let { File(it).parentFile }
            for (name in listOf("carryover.log", "carryover.1.log")) {
                val f = logs?.let { File(it, name) } ?: continue
                if (f.isFile) add(name, f.readBytes())
            }
            add("android.log", logcat())
        }
        zip
    }

    /** A content:// URI other apps may read for a zip from [build]. */
    fun uri(ctx: Context, zip: File): Uri = FileProvider.getUriForFile(ctx, "${ctx.packageName}.files", zip)

    /** The share sheet for a zip from [build]. */
    fun shareIntent(ctx: Context, zip: File, chooserTitle: String): Intent {
        val uri = uri(ctx, zip)
        val send = Intent(Intent.ACTION_SEND)
            .setType("application/zip")
            .putExtra(Intent.EXTRA_STREAM, uri)
            .putExtra(Intent.EXTRA_SUBJECT, "Carryover logs")
            .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        send.clipData = android.content.ClipData.newRawUri(zip.name, uri)
        return Intent.createChooser(send, chooserTitle)
    }

    /** Copies a zip from [build] to a document the user chose (Save to a file). */
    suspend fun save(ctx: Context, zip: File, target: Uri) = withContext(Dispatchers.IO) {
        val out = ctx.contentResolver.openOutputStream(target, "wt") ?: error("can't write $target")
        out.use { o -> zip.inputStream().use { it.copyTo(o) } }
    }

    /** What a bug report needs first: versions, device, permissions. English, for the developers. */
    private fun report(ctx: Context): String {
        val pkg = runCatching { ctx.packageManager.getPackageInfo(ctx.packageName, 0) }.getOrNull()
        val notifications = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        @Suppress("DEPRECATION")
        val versionCode = pkg?.let { if (Build.VERSION.SDK_INT >= 28) it.longVersionCode else it.versionCode.toLong() }
        return buildString {
            appendLine("Carryover for Android")
            appendLine("App: ${pkg?.versionName} ($versionCode)")
            appendLine("Core: ${runCatching { Core.version }.getOrDefault("?")}")
            Core.initError?.let { appendLine("Core failed to start: $it") }
            appendLine("Android: ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
            appendLine("Device: ${Build.MANUFACTURER} ${Build.MODEL}")
            appendLine("ABIs: ${Build.SUPPORTED_ABIS.joinToString()}")
            appendLine("Language: ${AppLanguage.current(ctx).ifEmpty { "system (${ctx.resources.configuration.locales[0]})" }}")
            appendLine("All-files access: ${Storage.hasAccess(ctx)}")
            appendLine("Usage access: ${hasUsageAccess(ctx)}")
            appendLine("Notifications: $notifications")
            appendLine("Collected: ${LocalDateTime.now()}")
        }
    }

    /** Android's log of this process (this run of the app only); empty if it can't be read. */
    private fun logcat(): ByteArray = runCatching {
        val p = ProcessBuilder("logcat", "-d", "-v", "threadtime", "--pid=${Process.myPid()}")
            .redirectErrorStream(true)
            .start()
        val bytes = p.inputStream.use { it.readBytes() }
        p.waitFor()
        if (bytes.size > MAX_LOGCAT) bytes.copyOfRange(bytes.size - MAX_LOGCAT, bytes.size) else bytes
    }.getOrElse { "logcat could not be read: $it\n".toByteArray() }
}
