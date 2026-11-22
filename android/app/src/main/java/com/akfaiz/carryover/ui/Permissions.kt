package com.akfaiz.carryover.ui

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect
import com.akfaiz.carryover.core.Storage
import com.akfaiz.carryover.core.hasUsageAccess
import com.akfaiz.carryover.core.usageAccessIntent

/**
 * What the user allowed Carryover, rechecked whenever the app comes back to
 * the front (the grants happen in the system's settings), and how to ask for
 * each.
 */
class Permissions(
    val storage: Boolean,
    val usage: Boolean,
    val notifications: Boolean,
    val askStorage: () -> Unit,
    val askUsage: () -> Unit,
    val askNotifications: () -> Unit,
) {
    companion object {
        /** Android 13+ sends notifications only once the user allowed them. */
        val notificationsAsked: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
    }
}

private fun notificationsAllowed(ctx: Context): Boolean =
    Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
        ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED

@Composable
fun rememberPermissions(): Permissions {
    val ctx = LocalContext.current
    var storage by remember { mutableStateOf(Storage.hasAccess(ctx)) }
    var usage by remember { mutableStateOf(hasUsageAccess(ctx)) }
    var notifications by remember { mutableStateOf(notificationsAllowed(ctx)) }
    LifecycleResumeEffect(Unit) {
        storage = Storage.hasAccess(ctx)
        usage = hasUsageAccess(ctx)
        notifications = notificationsAllowed(ctx)
        onPauseOrDispose {}
    }
    val legacyStorage = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) {
        storage = Storage.hasAccess(ctx)
    }
    val notify = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        notifications = granted
        // Asked before and turned down: Android doesn't ask again, its settings page can.
        if (!granted) ctx.startActivity(notificationSettingsIntent(ctx))
    }
    return Permissions(
        storage = storage,
        usage = usage,
        notifications = notifications,
        askStorage = {
            if (Storage.usesAllFilesAccess) {
                ctx.startActivity(Storage.allFilesAccessIntent(ctx))
            } else {
                legacyStorage.launch(Storage.legacyPermissions)
            }
        },
        askUsage = { ctx.startActivity(usageAccessIntent(ctx)) },
        askNotifications = {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                notify.launch(Manifest.permission.POST_NOTIFICATIONS)
            } else {
                ctx.startActivity(notificationSettingsIntent(ctx))
            }
        },
    )
}

private fun notificationSettingsIntent(ctx: Context): Intent =
    Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, ctx.packageName)
