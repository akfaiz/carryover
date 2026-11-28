package com.akfaiz.carryover.ui.emulators

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.SportsEsports
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorPresets
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.TrashView
import com.akfaiz.carryover.core.isInstalled
import com.akfaiz.carryover.ui.components.AppIcon
import com.akfaiz.carryover.ui.components.AppPickerDialog
import com.akfaiz.carryover.ui.components.FolderField
import com.akfaiz.carryover.ui.components.SettingRow
import com.akfaiz.carryover.ui.components.appLabel
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.ChoiceRow
import com.akfaiz.carryover.ui.kit.EmptyState
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState

/** The emulators of the library, and which app each one is on this device. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EmulatorsScreen(state: UiState, vm: AppViewModel, nav: Nav) {
    var adding by rememberSaveable { mutableStateOf(false) }
    var showOthers by rememberSaveable { mutableStateOf(false) }
    val (usable, others) = state.emulators.partition(EmulatorPresets::onAndroid)
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(stringResource(R.string.nav_emulators)) },
        floatingActionButton = {
            // With no emulators the empty page has the button already.
            if (!state.loaded || usable.isNotEmpty()) ExtendedFloatingActionButton(
                onClick = { adding = true },
                icon = { Icon(Icons.Outlined.Add, contentDescription = null) },
                text = { Text(stringResource(R.string.action_add_emulator)) },
            )
        },
    ) { padding ->
        Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.TopCenter) {
            LazyColumn(
                Modifier.widthIn(max = 760.dp).fillMaxSize(),
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 96.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                item { Hint(stringResource(R.string.emulators_intro)) }
                if (state.loaded && usable.isEmpty()) {
                    item {
                        EmptyState(
                            Icons.Outlined.SportsEsports,
                            stringResource(R.string.emulators_empty_title),
                            stringResource(R.string.emulators_empty),
                            actions = {
                                AppButton(onClick = { adding = true }, icon = Icons.Outlined.Add) { Text(stringResource(R.string.action_add_emulator)) }
                            },
                        )
                    }
                }
                if (usable.isNotEmpty() || others.isNotEmpty()) {
                    item {
                        SectionCard(null, contentPadding = PaddingValues(vertical = 4.dp), gap = 0.dp, grows = true) {
                            for (e in usable) EmulatorRow(e) { nav.go(Screen.Emulator(e.id)) }
                            // Emulators without an Android app (Ryujinx, Cemu, …) stay out of the way,
                            // still reachable in case one has an Android version after all.
                            if (others.isNotEmpty()) {
                                SettingRow(
                                    if (showOthers) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility,
                                    if (showOthers) {
                                        stringResource(R.string.emulators_hide_others)
                                    } else {
                                        pluralStringResource(R.plurals.emulators_show_others, others.size, others.size)
                                    },
                                    stringResource(R.string.emulators_others_hint),
                                    onClick = { showOthers = !showOthers },
                                    trailing = null,
                                )
                                if (showOthers) {
                                    for (e in others) EmulatorRow(e) { nav.go(Screen.Emulator(e.id)) }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    if (adding) {
        AppPickerDialog(stringResource(R.string.emulators_choose_app), onDismiss = { adding = false }) { app ->
            adding = false
            val preset = EmulatorPresets.forPackage(app.pkg)
            // Straight to its page: how a game starts and the data folder are set there.
            vm.saveEmulator("", preset?.name ?: app.label, EmulatorPresets.exeFor(preset, app.pkg), "", "") { id ->
                if (id.isNotEmpty()) nav.go(Screen.Emulator(id))
            }
        }
    }
}

@Composable
private fun EmulatorRow(e: EmulatorView, open: () -> Unit) {
    val ctx = LocalContext.current
    val exe = e.device.exe
    val games = pluralStringResource(R.plurals.emulator_games, e.games, e.games)
    val detail = when {
        exe.isBlank() -> stringResource(R.string.emulator_not_set_up)
        !isInstalled(ctx.packageManager, exe) -> stringResource(R.string.emulator_not_installed, exe.substringBefore('/'))
        else -> appLabel(ctx, exe)
    }
    SettingRow(
        icon = null,
        title = e.name,
        detail = "$detail · $games",
        onClick = open,
        leading = { AppIcon(exe.substringBefore('/'), 40) },
    )
}

/**
 * This device's setup of one emulator: which app it is, how a game starts in
 * it (open the app and pick the game there, or start the game directly with a
 * template) and its data folder.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EmulatorScreen(id: String, state: UiState, vm: AppViewModel, nav: Nav) {
    val e = state.emulators.firstOrNull { it.id == id }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(e?.name ?: stringResource(R.string.nav_emulators), nav::back) },
    ) { padding ->
        if (e == null) return@Scaffold
        PageColumn(padding) {
            EmulatorEditor(e, vm, nav, Modifier.fillMaxWidth())
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun EmulatorEditor(e: EmulatorView, vm: AppViewModel, nav: Nav, modifier: Modifier) {
    val ctx = LocalContext.current
    var exe by remember(e.id, e.device.exe) { mutableStateOf(e.device.exe) }
    var args by remember(e.id, e.device.args) { mutableStateOf(e.device.args) }
    var direct by remember(e.id, e.device.args) { mutableStateOf(e.device.args.isNotBlank()) }
    var dataDir by remember(e.id, e.device.dataDir) { mutableStateOf(e.device.dataDir) }
    var picking by rememberSaveable { mutableStateOf(false) }
    var removing by rememberSaveable { mutableStateOf(false) }
    var deleting by rememberSaveable { mutableStateOf(false) }
    val preset = EmulatorPresets.forPackage(exe.substringBefore('/'))
    val dirty = exe != e.device.exe || (if (direct) args else "") != e.device.args || dataDir != e.device.dataDir

    Column(modifier, verticalArrangement = Arrangement.spacedBy(16.dp)) {
        SectionCard(stringResource(R.string.emulator_app)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                AppIcon(exe.substringBefore('/'), 44)
                Column(Modifier.weight(1f)) {
                    Text(
                        when {
                            exe.isBlank() -> stringResource(R.string.emulator_not_set_up)
                            !isInstalled(ctx.packageManager, exe) -> stringResource(R.string.emulator_not_installed, exe.substringBefore('/'))
                            else -> appLabel(ctx, exe)
                        },
                        style = MaterialTheme.typography.bodyLarge,
                    )
                    if (exe.isNotBlank()) {
                        Hint(exe)
                    }
                }
            }
            AppButton(onClick = { picking = true }, kind = BtnKind.Secondary) {
                Text(stringResource(if (exe.isBlank()) R.string.action_choose_app else R.string.action_change_app))
            }
        }
        if (exe.isNotBlank()) {
            SectionCard(stringResource(R.string.game_launch)) {
                Choice(stringResource(R.string.emulator_mode_open), !direct) { direct = false }
                Choice(stringResource(R.string.emulator_mode_direct), direct) {
                    direct = true
                    if (args.isBlank()) args = preset?.template.orEmpty()
                }
                if (direct) {
                    val note = preset?.note?.takeIf { it != 0 }?.let { stringResource(it) + " " }.orEmpty()
                    OutlinedTextField(
                        args,
                        { args = it },
                        label = { Text(stringResource(R.string.emulator_template)) },
                        supportingText = { Text(note + stringResource(R.string.emulator_template_hint)) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            val switch = e.preset == "switch" || preset?.manifestPreset == "switch"
            SectionCard(stringResource(if (switch) R.string.emulator_data_folder else R.string.emulator_data_folder_optional)) {
                preset?.savesNote?.takeIf { it != 0 }?.let { Hint(stringResource(it)) }
                FolderField(
                    dataDir,
                    { dataDir = it },
                    stringResource(R.string.emulator_data_folder),
                    supporting = stringResource(if (switch) R.string.emulator_data_switch_hint else R.string.emulator_data_hint),
                )
            }
        }
        // Wraps onto a second line when the labels are long (Bahasa Indonesia on a phone).
        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
            itemVerticalAlignment = Alignment.CenterVertically,
        ) {
            AppButton(
                onClick = { vm.saveEmulator(e.id, e.name, exe, if (direct) args else "", dataDir) },
                enabled = dirty,
            ) { Text(stringResource(R.string.action_save)) }
            if (e.setUp && e.device.exe.isNotBlank()) {
                AppButton(onClick = { nav.go(Screen.Frontends(e.id)) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_use_frontend)) }
            }
        }
        // Apart from Save, each with what it does: one is this device only, the other every device.
        SectionCard(stringResource(R.string.emulator_danger_title)) {
            if (e.setUp) {
                DangerAction(stringResource(R.string.action_remove_device), stringResource(R.string.emulator_remove_hint)) { removing = true }
            }
            DangerAction(stringResource(R.string.action_delete_emulator), stringResource(R.string.emulator_delete_hint)) { deleting = true }
        }
    }
    if (removing) {
        AlertDialog(
            onDismissRequest = { removing = false },
            title = { Text(stringResource(R.string.emulator_remove_title, e.name)) },
            text = { Text(stringResource(R.string.emulator_remove_text, e.name)) },
            confirmButton = {
                AppButton(onClick = {
                    removing = false
                    vm.unsetEmulator(e.id, e.name)
                    nav.back()
                }, kind = BtnKind.Danger) { Text(stringResource(R.string.action_remove)) }
            },
            dismissButton = { AppButton(onClick = { removing = false }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
    if (deleting) {
        AlertDialog(
            onDismissRequest = { deleting = false },
            title = { Text(stringResource(R.string.dialog_delete_game_title, e.name)) },
            text = { Text(stringResource(R.string.dialog_delete_emulator_text, TrashView().days)) },
            confirmButton = {
                AppButton(
                    onClick = {
                        deleting = false
                        vm.deleteEmulator(e.id, e.name) { nav.back() }
                    },
                    kind = BtnKind.Danger,
                ) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { AppButton(onClick = { deleting = false }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
    if (picking) {
        AppPickerDialog(stringResource(R.string.game_choose_app_title, e.name), onDismiss = { picking = false }) { app ->
            picking = false
            val p = EmulatorPresets.forPackage(app.pkg)
            exe = EmulatorPresets.exeFor(p, app.pkg)
            if (direct && p != null) args = p.template
        }
    }
}

/** A destructive action: its button, and a line on what it does. */
@Composable
private fun DangerAction(label: String, hint: String, onClick: () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        AppButton(onClick = onClick, kind = BtnKind.Danger) { Text(label) }
        Hint(hint)
    }
}

@Composable
private fun Choice(label: String, selected: Boolean, onClick: () -> Unit) {
    ChoiceRow(selected, onClick) { Text(label, style = MaterialTheme.typography.bodyMedium) }
}
