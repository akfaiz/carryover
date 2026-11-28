package com.akfaiz.carryover.ui.game

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorPresets
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.ui.components.AppPickerDialog
import com.akfaiz.carryover.ui.components.FolderField
import com.akfaiz.carryover.ui.components.GameFileField
import com.akfaiz.carryover.ui.components.appLabel
import com.akfaiz.carryover.ui.components.rememberAppIcon
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState

/** The ways [g] is played on this device, as the game's page lists them (emulators with an Android app). */
fun androidProfiles(g: GameView, state: UiState): List<Profile> {
    val here = state.emulators.filter(EmulatorPresets::onAndroid).map { it.id }.toSet()
    return g.forAndroid(here)?.profiles.orEmpty()
}

/**
 * Edits one way a game is played here: its save locations (this device's
 * folder for each, which files sync, adding and removing them) and how it
 * starts (the game file for an emulator, the app for the game's own saves).
 * The locations are the game's on every device; a profile that uses another
 * emulator's saves only chooses its own folders.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ProfileEditCard(g: GameView, p: Profile, state: UiState, vm: AppViewModel, nav: Nav) {
    // Edited paths by slot key, starting from the stored ones.
    val edits = remember(p.key, p.paths) { mutableStateMapOf<String, String>().apply { putAll(p.paths) } }
    val dirty = p.slots.any { (edits[it.key] ?: "") != (p.paths[it.key] ?: "") }
    val sharing = p.shareWith.isNotEmpty()
    // The location being edited ("" for a new one) or removed; null while no dialog is open.
    var editing by remember(p.key) { mutableStateOf<String?>(null) }
    var removing by remember(p.key) { mutableStateOf<SaveSlot?>(null) }
    val suggest: SuggestViewModel = viewModel()
    val suggestState by suggest.state.collectAsStateWithLifecycle()
    if (suggestState.key == p.key) SaveSuggestDialog(suggestState, onUse = suggest::use, onClose = suggest::close)

    SectionCard(p.emulator ?: stringResource(R.string.game_profile_own)) {
        Text(stringResource(R.string.game_save_folders), style = MaterialTheme.typography.labelLarge)
        if (sharing) {
            val shared = g.emulators.firstOrNull { it.emulatorId == p.shareWith }?.name ?: p.shareWith
            Hint(stringResource(R.string.edit_slots_shared, p.emulator.orEmpty(), shared))
        }
        // Some emulators keep saves where Carryover can't reach until the user picks a folder in them: say how,
        // until the folders are set.
        if (!p.canSync) {
            EmulatorPresets.savesNote(state.emulators.firstOrNull { it.id == p.emulatorId }).takeIf { it != 0 }?.let { Hint(stringResource(it)) }
        }
        for (slot in p.slots) {
            val err = p.pathErrors[slot.key]
            val resolved = p.resolvedPaths[slot.key]
            FolderField(
                value = edits[slot.key] ?: "",
                onChange = { edits[slot.key] = it },
                label = slot.title,
                supporting = err ?: resolved?.takeIf { it != edits[slot.key] }?.let { "→ $it" },
                isError = err != null,
                note = filtersSummary(slot),
                actions = if (sharing) {
                    null
                } else {
                    {
                        IconButton(onClick = { editing = slot.key }) {
                            Icon(Icons.Outlined.Edit, contentDescription = stringResource(R.string.edit_slot_rename))
                        }
                        // The last one stays: a profile with no location has nothing to sync.
                        if (p.slots.size > 1) {
                            IconButton(onClick = { removing = slot }) {
                                Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.action_remove))
                            }
                        }
                    }
                },
            )
        }
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), itemVerticalAlignment = Alignment.CenterVertically) {
            AnimatedVisibility(dirty) {
                AppButton(onClick = { vm.setPaths(p.key, p.slots.associate { it.key to (edits[it.key] ?: "") }) }) {
                    Text(stringResource(R.string.action_save))
                }
            }
            // Where the emulator keeps this game's saves, from its manifest (needs its type and data folder).
            if (state.emulators.firstOrNull { it.id == p.emulatorId }?.preset.orEmpty().isNotEmpty()) {
                AppButton(onClick = { suggest.open(p.key) }, kind = BtnKind.Ghost, icon = Icons.Outlined.Search) {
                    Text(stringResource(R.string.game_suggest_saves))
                }
            }
            if (!sharing) {
                AppButton(onClick = { editing = "" }, kind = BtnKind.Ghost, icon = Icons.Outlined.Add) {
                    Text(stringResource(R.string.action_add_slot))
                }
            }
        }
        HorizontalDivider(Modifier.padding(vertical = 4.dp), color = MaterialTheme.colorScheme.outlineVariant)
        LaunchEditor(g, p, state, vm, nav)
    }

    editing?.let { key ->
        val current = p.slots.firstOrNull { it.key == key }
        SlotDialog(
            title = stringResource(if (current == null) R.string.edit_slot_new else R.string.edit_slot_rename),
            initial = current,
            onDismiss = { editing = null },
        ) { edited ->
            editing = null
            // Existing slots go back with their keys, a new one with no key.
            vm.setSlots(p.key, if (current == null) p.slots + edited else p.slots.map { if (it.key == key) edited else it })
        }
    }
    removing?.let { sl ->
        AlertDialog(
            onDismissRequest = { removing = null },
            title = { Text(stringResource(R.string.edit_slot_remove_title, sl.title)) },
            text = { Text(stringResource(R.string.edit_slot_remove_text, sl.title)) },
            confirmButton = {
                AppButton(onClick = {
                    removing = null
                    vm.setSlots(p.key, p.slots.filter { it.key != sl.key })
                }, kind = BtnKind.Danger) { Text(stringResource(R.string.action_remove)) }
            },
            dismissButton = { AppButton(onClick = { removing = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/** How the profile starts here: the game's own app, or its emulator (set up, or not yet) and game file. */
@Composable
private fun LaunchEditor(g: GameView, p: Profile, state: UiState, vm: AppViewModel, nav: Nav) {
    var picking by rememberSaveable { mutableStateOf(false) }
    Text(stringResource(R.string.game_launch), style = MaterialTheme.typography.labelLarge)
    ProfileAppLine(p, state)
    if (p.emulatorId == null) {
        AppButton(onClick = { picking = true }, kind = BtnKind.Secondary) {
            Text(stringResource(if (p.app.isBlank()) R.string.action_choose_game_app else R.string.action_change_app))
        }
    } else {
        val emu = state.emulators.firstOrNull { it.id == p.emulatorId }
        if (emu == null || emu.device.exe.isBlank()) {
            AppButton(onClick = { nav.go(if (emu != null) Screen.Emulator(emu.id) else Screen.Emulators) }, kind = BtnKind.Secondary) {
                Text(stringResource(R.string.action_set_up_emulators))
            }
        } else if (emu.device.args.isNotBlank()) {
            var rom by remember(p.key, p.rom) { mutableStateOf(p.rom) }
            GameFileField(rom, { rom = it }, stringResource(R.string.game_file))
            AnimatedVisibility(rom != p.rom) {
                AppButton(onClick = { vm.setRom(p.key, rom) }) { Text(stringResource(R.string.action_save_game_file)) }
            }
        }
    }
    if (picking) {
        AppPickerDialog(stringResource(R.string.game_choose_app_title, g.name), onDismiss = { picking = false }) { app ->
            picking = false
            vm.setNativeApp(g.id, app.pkg)
        }
    }
}

/** Which app plays the profile here: the game's own, or its emulator's ("PPSSPP", or "RetroArch: RetroArch Plus"). */
@Composable
fun ProfileAppLine(p: Profile, state: UiState) {
    val ctx = LocalContext.current
    if (p.emulatorId == null) {
        AppLine(
            p.app.substringBefore('/'),
            if (p.app.isBlank()) stringResource(R.string.game_app_none) else stringResource(R.string.game_app, appLabel(ctx, p.app)),
        )
        return
    }
    val exe = state.emulators.firstOrNull { it.id == p.emulatorId }?.device?.exe.orEmpty()
    AppLine(
        exe.substringBefore('/'),
        if (exe.isBlank()) {
            stringResource(R.string.game_emulator_missing, p.emulator.orEmpty())
        } else {
            // "PPSSPP", not "PPSSPP: PPSSPP", when the emulator is named after its app.
            val app = appLabel(ctx, exe)
            if (app.equals(p.emulator.orEmpty(), ignoreCase = true)) app else stringResource(R.string.game_emulator_app, p.emulator.orEmpty(), app)
        },
    )
}

/** An app's icon (when installed) and a line about it. */
@Composable
private fun AppLine(pkg: String, text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        val icon = rememberAppIcon(pkg)
        if (icon != null) {
            Image(icon, null, Modifier.size(28.dp))
        } else {
            Icon(Icons.Outlined.Apps, null, Modifier.size(28.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(text, style = MaterialTheme.typography.bodyMedium)
    }
}
