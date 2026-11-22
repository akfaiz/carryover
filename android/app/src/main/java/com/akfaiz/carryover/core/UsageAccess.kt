package com.akfaiz.carryover.core

import android.app.AppOpsManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Process
import android.provider.Settings

/** Whether the user let Carryover see which app is in front (Usage access). */
@Suppress("DEPRECATION") // no undeprecated check covers API 26-36
fun hasUsageAccess(ctx: Context): Boolean {
    val ops = ctx.getSystemService(Context.APP_OPS_SERVICE) as AppOpsManager
    val mode = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
        ops.unsafeCheckOpNoThrow(AppOpsManager.OPSTR_GET_USAGE_STATS, Process.myUid(), ctx.packageName)
    } else {
        ops.checkOpNoThrow(AppOpsManager.OPSTR_GET_USAGE_STATS, Process.myUid(), ctx.packageName)
    }
    return mode == AppOpsManager.MODE_ALLOWED
}

/**
 * Carryover's App info page, where Android 13+ unlocks restricted settings
 * (⋮ → Allow restricted settings) for an app installed from a downloaded APK.
 */
fun appInfoIntent(ctx: Context): Intent =
    Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:${ctx.packageName}"))

/** The settings page that grants Usage access (this app's entry where the system has one). */
fun usageAccessIntent(ctx: Context): Intent {
    val own = Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS, Uri.parse("package:${ctx.packageName}"))
    return if (own.resolveActivity(ctx.packageManager) != null) own else Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS)
}
