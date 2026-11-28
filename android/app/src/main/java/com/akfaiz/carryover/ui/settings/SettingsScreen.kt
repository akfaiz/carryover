package com.akfaiz.carryover.ui.settings

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.os.Build
import android.provider.DocumentsContract
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Launch
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Contrast
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.Group
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Language
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.Palette
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.QueryStats
import androidx.compose.material.icons.outlined.SaveAlt
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.Timer
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.AppLanguage
import com.akfaiz.carryover.core.BugReport
import com.akfaiz.carryover.core.DeviceView
import com.akfaiz.carryover.ui.components.Permissions
import com.akfaiz.carryover.ui.components.SectionHeader
import com.akfaiz.carryover.ui.components.SettingRow
import com.akfaiz.carryover.ui.components.relativeTime
import com.akfaiz.carryover.ui.components.rememberPermissions
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.ChoiceRow
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.Pill
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.Spinner
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.backupChoices
import java.io.File
import kotlinx.coroutines.launch

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
    // Sent here by a page that needs the key (Find cover): ask for it straight away.
    LaunchedEffect(nav.askGridKey) {
        if (nav.askGridKey) {
            gridKey = true
            nav.askGridKey = false
        }
    }
    var backupsDialog by rememberSaveable { mutableStateOf(false) }
    var devicesDialog by rememberSaveable { mutableStateOf(false) }
    val devices by vm.devices.collectAsStateWithLifecycle()
    // Read each time the page opens: another device may have changed the library meanwhile.
    LaunchedEffect(Unit) { vm.loadDevices() }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(stringResource(R.string.nav_settings)) },
    ) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(Modifier.widthIn(max = 760.dp).fillMaxWidth().padding(horizontal = 16.dp).padding(bottom = 24.dp)) {
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
                        detailLines = Int.MAX_VALUE,
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
                        Icons.Outlined.Devices,
                        stringResource(R.string.setting_devices),
                        stringResource(
                            if (devices.any { it.sameName }) R.string.setting_devices_earlier else R.string.setting_devices_hint,
                        ),
                        onClick = { devicesDialog = true },
                    )
                    SettingRow(
                        Icons.AutoMirrored.Outlined.Launch,
                        stringResource(R.string.frontends_title),
                        stringResource(R.string.frontends_row_hint),
                        onClick = { nav.go(Screen.Frontends()) },
                    )
                }

                UpdateSection(state, vm)

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
    if (devicesDialog) {
        DevicesDialog(devices.filterNot { it.isThis }, vm, onDismiss = { devicesDialog = false })
    }
}

/** A device's name, or a stand-in when it has none. */
@Composable
private fun deviceName(d: DeviceView): String = d.name.ifEmpty { stringResource(R.string.device_unnamed) }

/**
 * The other devices the library holds settings of: take back an earlier install's settings ("This is me", for a
 * phone that was reinstalled) or forget a device that is gone. Each asks first.
 */
@Composable
private fun DevicesDialog(others: List<DeviceView>, vm: SettingsViewModel, onDismiss: () -> Unit) {
    var reuse by rememberSaveable { mutableStateOf<String?>(null) }
    var forget by rememberSaveable { mutableStateOf<String?>(null) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_devices)) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Hint(stringResource(R.string.devices_intro))
                if (others.isEmpty()) Text(stringResource(R.string.devices_none))
                others.forEach { d -> DeviceItem(d, onReuse = { reuse = d.id }, onForget = { forget = d.id }) }
            }
        },
        confirmButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_close)) } },
    )
    others.firstOrNull { it.id == reuse }?.let { d ->
        val name = deviceName(d)
        ConfirmDeviceDialog(
            stringResource(R.string.dialog_reuse_device_title, name),
            stringResource(R.string.dialog_reuse_device_text, name),
            stringResource(R.string.action_this_is_me),
            danger = false,
            onConfirm = {
                reuse = null
                onDismiss()
                vm.reuseDevice(d, name)
            },
            onDismiss = { reuse = null },
        )
    }
    others.firstOrNull { it.id == forget }?.let { d ->
        ConfirmDeviceDialog(
            stringResource(R.string.dialog_forget_device_title, deviceName(d)),
            stringResource(R.string.dialog_forget_device_text),
            stringResource(R.string.action_forget),
            danger = true,
            onConfirm = {
                forget = null
                vm.forgetDevice(d)
            },
            onDismiss = { forget = null },
        )
    }
}

@Composable
private fun DeviceItem(d: DeviceView, onReuse: () -> Unit, onForget: () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(deviceName(d), style = MaterialTheme.typography.titleSmall)
        if (d.sameName) Pill(stringResource(R.string.device_same_name), Tone.Info)
        Hint(stringResource(R.string.device_line, relativeTime(d.lastSeen), d.games, d.emulators))
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            AppButton(onClick = onReuse, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_this_is_me)) }
            AppButton(onClick = onForget, kind = BtnKind.Danger) { Text(stringResource(R.string.action_forget)) }
        }
    }
}

@Composable
private fun ConfirmDeviceDialog(
    title: String,
    text: String,
    action: String,
    danger: Boolean,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { Text(text) },
        confirmButton = {
            AppButton(onClick = onConfirm, kind = if (danger) BtnKind.Danger else BtnKind.Primary) { Text(action) }
        },
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
    )
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
        // Under the text of the row above: 16dp padding, a 24dp icon and a 16dp gap in.
        Modifier.padding(start = 56.dp, end = 16.dp, bottom = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AppButton(
            onClick = { collect { ctx.startActivity(BugReport.shareIntent(ctx, it, chooserTitle)) } },
            kind = BtnKind.Secondary,
            icon = Icons.Outlined.Share,
            enabled = !busy,
        ) { Text(stringResource(R.string.action_share)) }
        AppButton(onClick = { collect { saver.launch(it.name) } }, kind = BtnKind.Ghost, icon = Icons.Outlined.SaveAlt, enabled = !busy) {
            Text(stringResource(R.string.action_save_file))
        }
        if (busy) Spinner(small = true)
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
internal fun Group(content: @Composable ColumnScope.() -> Unit) {
    SectionCard(null, contentPadding = PaddingValues(vertical = 4.dp), gap = 0.dp, content = content)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ThemeChoice(theme: String, onChange: (String) -> Unit) = SegmentedChoice(
    listOf("" to R.string.theme_system, "light" to R.string.theme_light, "dark" to R.string.theme_dark),
    theme,
    onChange,
)

/** A row of segments under a setting's text: [options] are values with their labels. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SegmentedChoice(options: List<Pair<String, Int>>, chosen: String, onChange: (String) -> Unit) {
    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(start = 56.dp, end = 16.dp, bottom = 12.dp)) {
        options.forEachIndexed { i, (value, label) ->
            SegmentedButton(
                selected = chosen == value,
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
                Pill(stringResource(R.string.perm_granted), Tone.Ok, icon = Icons.Outlined.CheckCircle)
            } else {
                AppButton(onClick = ask) { Text(stringResource(R.string.action_allow)) }
            }
        },
    )
}

@Composable
private fun languageName(tag: String): String = when (tag) {
    "en" -> stringResource(R.string.language_en)
    "es" -> stringResource(R.string.language_es)
    "id" -> stringResource(R.string.language_id)
    "pt-BR" -> stringResource(R.string.language_pt_br)
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
                    ChoiceRow(current == tag, { onPick(tag) }) { Text(languageName(tag)) }
                }
            }
        },
        confirmButton = {},
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
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
                Hint(stringResource(R.string.dialog_backups_text), Modifier.padding(bottom = 8.dp))
                for (keep in backupChoices(current)) {
                    ChoiceRow(current == keep, { onPick(keep) }) {
                        Text(if (keep == 0) stringResource(R.string.backups_choice_off) else pluralStringResource(R.plurals.backups_keep, keep, keep))
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
    )
}

@Composable
private fun GraceDialog(choices: List<Int>, current: Int, onDismiss: () -> Unit, onPick: (Int) -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.setting_grace)) },
        text = {
            Column {
                Hint(stringResource(R.string.setting_grace_hint), Modifier.padding(bottom = 8.dp))
                for (seconds in choices) {
                    ChoiceRow(current == seconds, { onPick(seconds) }) { Text(stringResource(R.string.grace_seconds, seconds)) }
                }
            }
        },
        confirmButton = {},
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
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
            AppButton(onClick = { onSave(name) }, enabled = name.isNotBlank() && name != current) {
                Text(stringResource(R.string.action_save))
            }
        },
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
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
                    Spinner(Modifier.padding(12.dp))
                } else {
                    AppButton(
                        onClick = {
                            busy = true
                            onSave(key, finish)
                        },
                        enabled = key.isNotBlank(),
                    ) { Text(stringResource(R.string.action_save)) }
                    if (hasKey) {
                        AppButton(
                            onClick = {
                                busy = true
                                onSave("", finish)
                            },
                            kind = BtnKind.Danger,
                        ) { Text(stringResource(R.string.action_remove_key)) }
                    }
                    AppButton(onClick = onGetKey, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_get_key)) }
                    AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) }
                }
            }
        },
    )
}
