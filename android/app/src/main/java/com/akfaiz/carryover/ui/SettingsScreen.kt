package com.akfaiz.carryover.ui

import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.material.icons.outlined.Group
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Delete
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.os.Build
import android.provider.DocumentsContract
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Contrast
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.SaveAlt
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Language
import androidx.compose.material.icons.automirrored.outlined.Launch
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.Palette
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.QueryStats
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.Timer
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.size
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.runtime.rememberCoroutineScope
import com.akfaiz.carryover.core.BugReport
import java.io.File
import kotlinx.coroutines.launch
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.AppLanguage

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(nav: Nav, vm: SettingsViewModel = viewModel()) {
    val state by vm.state.collectAsStateWithLifecycle()
    val ctx = LocalContext.current
    val perms = rememberPermissions()
    var language by rememberSaveable { mutableStateOf(false) }
    var naming by rememberSaveable { mutableStateOf(false) }
    var graceDialog by rememberSaveable { mutableStateOf(false) }
    var gridKey by rememberSaveable { mutableStateOf(false) }
    var backupsDialog by rememberSaveable { mutableStateOf(false) }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { TopAppBar(title = { Text(stringResource(R.string.nav_settings)) }, windowInsets = pageBarInsets()) },
    ) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(Modifier.widthIn(max = 760.dp).fillMaxWidth().padding(horizontal = 12.dp).padding(bottom = 24.dp)) {
                SectionHeader(stringResource(R.string.section_cloud))
                Group {
                    SettingRow(
                        if (state.cloudReady) Icons.Outlined.Cloud else Icons.Outlined.CloudOff,
                        if (state.cloudReady) {
                            "${state.settings.remote}:${state.settings.remoteRoot.ifEmpty { "Carryover" }}"
                        } else {
                            stringResource(R.string.cloud_not_connected)
                        },
                        if (state.cloudReady) null else stringResource(R.string.setup_cloud_text),
                        onClick = { nav.go(Screen.Cloud) },
                    )
                    SettingRow(
                        Icons.Outlined.History,
                        stringResource(R.string.setting_backups),
                        if (state.backupsKept == 0) {
                            stringResource(R.string.setting_backups_off)
                        } else {
                            pluralStringResource(R.plurals.setting_backups_on, state.backupsKept, state.backupsKept)
                        },
                        onClick = { backupsDialog = true },
                    )
                    SettingRow(
                        Icons.Outlined.Group,
                        stringResource(R.string.setting_share_library),
                        stringResource(R.string.setting_share_library_hint),
                        onClick = { vm.setShareLibrary(!state.sharesLibrary) },
                        trailing = { Switch(checked = state.sharesLibrary, onCheckedChange = vm::setShareLibrary) },
                    )
                    // Covers are part of the library sync: they can only be shared while the library is.
                    SettingRow(
                        Icons.Outlined.Image,
                        stringResource(R.string.setting_sync_covers),
                        stringResource(if (state.sharesLibrary) R.string.setting_sync_covers_hint else R.string.setting_sync_covers_needs_share),
                        onClick = if (state.sharesLibrary) {
                            { vm.setSyncCovers(!state.syncsCovers) }
                        } else {
                            null
                        },
                        trailing = {
                            Switch(checked = state.syncsCovers, onCheckedChange = vm::setSyncCovers, enabled = state.sharesLibrary)
                        },
                    )
                    SettingRow(
                        Icons.Outlined.Delete,
                        stringResource(R.string.setting_trash),
                        stringResource(R.string.setting_trash_hint),
                        onClick = { nav.go(Screen.Trash) },
                    )
                }

                SectionHeader(stringResource(R.string.section_appearance))
                Group {
                    SettingRow(Icons.Outlined.Contrast, stringResource(R.string.setting_theme), trailing = null)
                    ThemeChoice(state.theme, vm::setTheme)
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                        SettingRow(
                            Icons.Outlined.Palette,
                            stringResource(R.string.setting_wallpaper),
                            stringResource(R.string.setting_wallpaper_hint),
                            onClick = { vm.setWallpaper(!state.wallpaper) },
                            trailing = { Switch(checked = state.wallpaper, onCheckedChange = vm::setWallpaper) },
                        )
                    }
                    SettingRow(
                        Icons.Outlined.Language,
                        stringResource(R.string.setting_language),
                        languageName(AppLanguage.current(ctx)),
                        onClick = { language = true },
                    )
                }

                SectionHeader(stringResource(R.string.section_permissions))
                Group {
                    PermissionRow(
                        Icons.Outlined.QueryStats,
                        R.string.perm_usage,
                        R.string.perm_usage_hint,
                        perms.usage,
                        perms.askUsage,
                    )
                    if (Permissions.notificationsAsked) {
                        PermissionRow(
                            Icons.Outlined.Notifications,
                            R.string.perm_notifications,
                            R.string.perm_notifications_hint,
                            perms.notifications,
                            perms.askNotifications,
                        )
                    }
                    PermissionRow(
                        Icons.Outlined.FolderOpen,
                        R.string.perm_storage,
                        R.string.perm_storage_hint,
                        perms.storage,
                        perms.askStorage,
                    )
                }

                SectionHeader(stringResource(R.string.section_artwork))
                Group {
                    SettingRow(
                        Icons.Outlined.Image,
                        stringResource(R.string.setting_grid_key),
                        stringResource(if (state.hasGridKey) R.string.setting_grid_key_set else R.string.setting_grid_key_unset),
                        onClick = { gridKey = true },
                    )
                }

                SectionHeader(stringResource(R.string.section_sessions))
                Group {
                    SettingRow(
                        Icons.Outlined.Timer,
                        stringResource(R.string.setting_grace),
                        stringResource(R.string.grace_seconds, state.graceSeconds),
                        onClick = { graceDialog = true },
                    )
                    SettingRow(
                        Icons.Outlined.Sync,
                        stringResource(R.string.setting_background_sync),
                        stringResource(R.string.setting_background_sync_hint),
                        onClick = { vm.setBackgroundSync(!state.backgroundSync) },
                        trailing = { Switch(checked = state.backgroundSync, onCheckedChange = vm::setBackgroundSync) },
                    )
                }

                SectionHeader(stringResource(R.string.section_device))
                Group {
                    SettingRow(
                        Icons.Outlined.PhoneAndroid,
                        stringResource(R.string.setting_device_name),
                        state.settings.deviceName,
                        onClick = { naming = true },
                    )
                    SettingRow(
                        Icons.AutoMirrored.Outlined.Launch,
                        stringResource(R.string.frontends_title),
                        stringResource(R.string.frontends_row_hint),
                        onClick = { nav.go(Screen.Frontends()) },
                    )
                }

                SectionHeader(stringResource(R.string.section_about))
                Group {
                    SettingRow(
                        Icons.Outlined.Info,
                        stringResource(R.string.about_version, appVersion(ctx)),
                        stringResource(R.string.about_core, vm.coreVersion),
                    )
                    LogsRow(vm)
                }
            }
        }
    }
    if (language) {
        LanguageDialog(AppLanguage.current(ctx), onDismiss = { language = false }) { tag ->
            language = false
            ctx.findActivity()?.let { AppLanguage.set(it, tag) }
        }
    }
    if (gridKey) {
        GridKeyDialog(
            hasKey = state.hasGridKey,
            onSave = vm::saveSteamGridKey,
            onGetKey = { vm.openBrowser("https://www.steamgriddb.com/profile/preferences/api") },
            onDismiss = { gridKey = false },
        )
    }
    if (backupsDialog) {
        BackupsDialog(state.backupsKept, onDismiss = { backupsDialog = false }) { keep ->
            backupsDialog = false
            vm.setBackupsKept(keep)
        }
    }
    if (graceDialog) {
        GraceDialog(state.graceChoices, state.graceSeconds, onDismiss = { graceDialog = false }) { seconds ->
            graceDialog = false
            vm.setGraceSeconds(seconds)
        }
    }
    if (naming) {
        DeviceNameDialog(state.settings.deviceName, onDismiss = { naming = false }) {
            naming = false
            vm.setDeviceName(it)
        }
    }
}

/**
 * The logs for a bug report: the log is in the app's private storage, so the
 * only way to it is a zip the user shares (share sheet) or saves (a file they
 * pick), see BugReport.
 */
@Composable
private fun LogsRow(vm: SettingsViewModel) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var zip by remember { mutableStateOf<File?>(null) }
    var busy by remember { mutableStateOf(false) }
    val chooserTitle = stringResource(R.string.logs_share_title)
    val saver = rememberLauncherForActivityResult(SaveInDownloads()) { uri ->
        val z = zip ?: return@rememberLauncherForActivityResult
        if (uri != null) vm.saveLogs(z, uri.toString())
    }
    fun collect(then: (File) -> Unit) {
        busy = true
        scope.launch {
            val f = vm.collectLogs()
            busy = false
            if (f != null) {
                zip = f
                then(f)
            }
        }
    }
    // All of the hint: it says what the logs may contain.
    SettingRow(
        Icons.Outlined.BugReport,
        stringResource(R.string.about_logs),
        stringResource(R.string.about_logs_hint),
        detailLines = Int.MAX_VALUE,
        trailing = null,
    )
    Row(
        Modifier.padding(start = 72.dp, end = 16.dp, bottom = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        FilledTonalButton(onClick = { collect { ctx.startActivity(BugReport.shareIntent(ctx, it, chooserTitle)) } }, enabled = !busy) {
            Icon(Icons.Outlined.Share, null, Modifier.size(18.dp))
            Text(stringResource(R.string.action_share), Modifier.padding(start = 8.dp))
        }
        OutlinedButton(onClick = { collect { saver.launch(it.name) } }, enabled = !busy) {
            Icon(Icons.Outlined.SaveAlt, null, Modifier.size(18.dp))
            Text(stringResource(R.string.action_save_file), Modifier.padding(start = 8.dp))
        }
        if (busy) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
    }
}

/**
 * Android's "save as" picker for a zip, opening in Downloads rather than the
 * folder last picked (often a game's save folder).
 */
private class SaveInDownloads : ActivityResultContracts.CreateDocument("application/zip") {
    override fun createIntent(context: Context, input: String): Intent = super.createIntent(context, input).putExtra(
        DocumentsContract.EXTRA_INITIAL_URI,
        DocumentsContract.buildDocumentUri("com.android.externalstorage.documents", "primary:Download"),
    )
}

/** Rows of one section on a card. */
@Composable
private fun Group(content: @Composable () -> Unit) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow),
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(Modifier.padding(vertical = 4.dp)) { content() }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ThemeChoice(theme: String, onChange: (String) -> Unit) {
    val options = listOf("" to R.string.theme_system, "light" to R.string.theme_light, "dark" to R.string.theme_dark)
    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(start = 56.dp, end = 16.dp, bottom = 12.dp)) {
        options.forEachIndexed { i, (value, label) ->
            SegmentedButton(
                selected = theme == value,
                onClick = { onChange(value) },
                shape = SegmentedButtonDefaults.itemShape(i, options.size),
                // The accent's container: the default fill is barely visible in the dark theme.
                colors = SegmentedButtonDefaults.colors(
                    activeContainerColor = MaterialTheme.colorScheme.primaryContainer,
                    activeContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                ),
                // No check mark: the filled segment shows the choice, and the mark
                // squeezed the label against the border of a narrow segment.
                icon = {},
            ) { Text(stringResource(label), maxLines = 1, overflow = TextOverflow.Ellipsis) }
        }
    }
}

@Composable
private fun PermissionRow(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: Int,
    hint: Int,
    granted: Boolean,
    ask: () -> Unit,
) {
    SettingRow(
        icon,
        stringResource(title),
        stringResource(hint),
        trailing = {
            if (granted) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Outlined.CheckCircle, null, tint = LocalStatusColors.current.ok)
                    Text(
                        stringResource(R.string.perm_granted),
                        style = MaterialTheme.typography.labelMedium,
                        color = LocalStatusColors.current.ok,
                        modifier = Modifier.padding(start = 4.dp),
                    )
                }
            } else {
                FilledTonalButton(onClick = ask) { Text(stringResource(R.string.action_allow)) }
            }
        },
    )
}

@Composable
private fun languageName(tag: String): String = when (tag) {
    "en" -> stringResource(R.string.language_en)
    "id" -> stringResource(R.string.language_id)
    else -> stringResource(R.string.language_system)
}

@Composable
private fun LanguageDialog(current: String, onDismiss: () -> Unit, onPick: (String) -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_language)) },
        text = {
            Column {
                for (tag in AppLanguage.choices) {
                    val shape = RoundedCornerShape(10.dp)
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .focusRing(shape)
                            .clip(shape)
                            .selectable(selected = current == tag, onClick = { onPick(tag) }),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(selected = current == tag, onClick = { onPick(tag) })
                        Text(languageName(tag))
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

/** How many cloud backups to keep per game: off, or the last few (the desktop app's setting too). */
@Composable
private fun BackupsDialog(current: Int, onDismiss: () -> Unit, onPick: (Int) -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_backups)) },
        text = {
            Column {
                Text(stringResource(R.string.dialog_backups_text), style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(bottom = 8.dp))
                for (keep in backupChoices(current)) {
                    val shape = RoundedCornerShape(10.dp)
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .focusRing(shape)
                            .clip(shape)
                            .selectable(selected = current == keep, onClick = { onPick(keep) }),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(selected = current == keep, onClick = { onPick(keep) })
                        Text(if (keep == 0) stringResource(R.string.backups_choice_off) else pluralStringResource(R.plurals.backups_keep, keep, keep))
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

@Composable
private fun GraceDialog(choices: List<Int>, current: Int, onDismiss: () -> Unit, onPick: (Int) -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_grace)) },
        text = {
            Column {
                Text(stringResource(R.string.setting_grace_hint), style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(bottom = 8.dp))
                for (seconds in choices) {
                    val shape = RoundedCornerShape(10.dp)
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .focusRing(shape)
                            .clip(shape)
                            .selectable(selected = current == seconds, onClick = { onPick(seconds) }),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(selected = current == seconds, onClick = { onPick(seconds) })
                        Text(stringResource(R.string.grace_seconds, seconds))
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

@Composable
private fun DeviceNameDialog(current: String, onDismiss: () -> Unit, onSave: (String) -> Unit) {
    var name by remember { mutableStateOf(current) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_device_name)) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    name,
                    { name = it },
                    singleLine = true,
                    supportingText = { Text(stringResource(R.string.setting_device_name_hint)) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            TextButton(onClick = { onSave(name) }, enabled = name.isNotBlank() && name != current) {
                Text(stringResource(R.string.action_save))
            }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

private fun appVersion(ctx: Context): String = runCatching {
    ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName
}.getOrNull().orEmpty()

fun Context.findActivity(): Activity? {
    var c: Context? = this
    while (c is ContextWrapper) {
        if (c is Activity) return c
        c = c.baseContext
    }
    return null
}

/**
 * The SteamGridDB API key: saved once SteamGridDB accepts it (the dialog stays
 * open, working, until it does). All the buttons are one column in the
 * confirm slot: a dialog with more than two buttons spaces them apart otherwise.
 */
@Composable
private fun GridKeyDialog(hasKey: Boolean, onSave: (String, (Boolean) -> Unit) -> Unit, onGetKey: () -> Unit, onDismiss: () -> Unit) {
    var key by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    val finish = { ok: Boolean ->
        busy = false
        if (ok) onDismiss()
    }
    AlertDialog(
        onDismissRequest = { if (!busy) onDismiss() },
        title = { Text(stringResource(R.string.setting_grid_key)) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(stringResource(R.string.dialog_grid_key_text), style = MaterialTheme.typography.bodyMedium)
                OutlinedTextField(
                    key,
                    { key = it },
                    label = { Text(stringResource(R.string.field_grid_key)) },
                    singleLine = true,
                    enabled = !busy,
                    visualTransformation = PasswordVisualTransformation(),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                if (busy) {
                    CircularProgressIndicator(Modifier.padding(12.dp).size(24.dp), strokeWidth = 2.dp)
                } else {
                    TextButton(
                        onClick = {
                            busy = true
                            onSave(key, finish)
                        },
                        enabled = key.isNotBlank(),
                    ) { Text(stringResource(R.string.action_save)) }
                    if (hasKey) {
                        TextButton(
                            onClick = {
                                busy = true
                                onSave("", finish)
                            },
                        ) { Text(stringResource(R.string.action_remove_key), color = MaterialTheme.colorScheme.error) }
                    }
                    TextButton(onClick = onGetKey) { Text(stringResource(R.string.action_get_key)) }
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) }
                }
            }
        },
    )
}
