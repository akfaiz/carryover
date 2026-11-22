package com.akfaiz.carryover.core

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.DocumentsContract
import android.provider.Settings

/**
 * Phase 1 reads save folders by path, so it needs access to all of shared
 * storage: All-files access on Android 11+, the storage permission before.
 * (Phase 4 replaces this with Storage Access Framework folders.)
 */
object Storage {
    /** The permission requested at runtime on Android 8-10. */
    val legacyPermissions = arrayOf(Manifest.permission.READ_EXTERNAL_STORAGE, Manifest.permission.WRITE_EXTERNAL_STORAGE)

    val usesAllFilesAccess: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R

    fun hasAccess(context: Context): Boolean =
        if (usesAllFilesAccess) {
            Environment.isExternalStorageManager()
        } else {
            context.checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
        }

    /** The settings page that grants All-files access (Android 11+). */
    fun allFilesAccessIntent(context: Context): Intent {
        val own = Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, Uri.parse("package:${context.packageName}"))
        return if (own.resolveActivity(context.packageManager) != null) {
            own
        } else {
            Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION)
        }
    }

    /**
     * The file path of a folder picked with ACTION_OPEN_DOCUMENT_TREE, or null
     * when it isn't on device storage (another app's provider, a cloud
     * folder), which phase 1 can't read by path.
     */
    fun treeUriToPath(uri: Uri): String? {
        if (uri.authority != "com.android.externalstorage.documents") return null
        val id = runCatching { DocumentsContract.getTreeDocumentId(uri) }.getOrNull() ?: return null
        val volume = id.substringBefore(':')
        val rel = id.substringAfter(':', "")
        val root = when (volume) {
            "primary" -> Environment.getExternalStorageDirectory().path
            "home" -> Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOCUMENTS).path
            else -> "/storage/$volume"
        }
        return if (rel.isEmpty()) root else "$root/$rel"
    }
}
