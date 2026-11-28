package com.akfaiz.carryover.ui.game

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorPresets
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.ui.components.AppIcon
import com.akfaiz.carryover.ui.components.appLabel
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.ChoiceRow
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.ListRows
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.dpadExit
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState

/**
 * Edits a game: its name and the emulators it is played with (for every
 * device), then for each way it is played here its save locations, this
 * device's folders for them and how it starts. An emulator added here may
 * share the saves of one the game already has (Lemon next to Eden: the same
 * Switch saves), so both sync one cloud folder. Emulators used only on other
 * devices (Ryujinx, the PC version) are listed but left alone.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditGameScreen(id: String, state: UiState, vm: AppViewModel, nav: Nav) {
    // The whole game, reloaded with the library (the library's copy leaves out other devices' emulators).
    val g by produceState<GameView?>(null, id, state.games) { value = vm.fullGame(id) }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(stringResource(R.string.edit_game), nav::back) },
    ) { padding ->
        val game = g ?: return@Scaffold
        PageColumn(padding) {
            NameCard(game, vm)
            EmulatorsCard(game, state, vm)
            // Each way it is played here: where its saves are and how it starts.
            for (p in androidProfiles(game, state)) {
                ProfileEditCard(game, p, state, vm, nav)
            }
        }
    }
}

@Composable
private fun NameCard(g: GameView, vm: AppViewModel) {
    var name by remember(g.id, g.name) { mutableStateOf(g.name) }
    SectionCard(null) {
        OutlinedTextField(
            name,
            { name = it },
            label = { Text(stringResource(R.string.field_name)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().dpadExit(),
        )
        AppButton(onClick = { vm.renameGame(g.id, name) }, enabled = name.isNotBlank() && name.trim() != g.name) {
            Text(stringResource(R.string.action_save))
        }
    }
}

@Composable
private fun EmulatorsCard(g: GameView, state: UiState, vm: AppViewModel) {
    val ctx = LocalContext.current
    var adding by rememberSaveable { mutableStateOf(false) }
    var removing by remember { mutableStateOf<ProfileView?>(null) }
    val byId = state.emulators.associateBy { it.id }
    val (here, elsewhere) = g.emulators.partition { p -> byId[p.emulatorId]?.let(EmulatorPresets::onAndroid) == true }
    val nameOf = { emuId: String -> byId[emuId]?.name ?: g.emulators.firstOrNull { it.emulatorId == emuId }?.name ?: emuId }

    SectionCard(stringResource(R.string.edit_played_with)) {
        if (here.isNotEmpty()) ListRows(here) { p ->
            val exe = byId[p.emulatorId]?.device?.exe.orEmpty()
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                AppIcon(exe.substringBefore('/'), 36)
                Column(Modifier.weight(1f)) {
                    Text(p.name, style = MaterialTheme.typography.bodyLarge)
                    Hint(
                        when {
                            p.shareWith.isNotEmpty() -> stringResource(R.string.edit_shares, nameOf(p.shareWith))
                            exe.isNotBlank() -> appLabel(ctx, exe)
                            else -> stringResource(R.string.emulator_not_set_up)
                        },
                    )
                }
                IconButton(onClick = { removing = p }) {
                    Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.action_remove))
                }
            }
        }
        // The game's own (PC) saves and emulators without an Android app stay on the other devices.
        val others = buildList {
            if (g.slots.isNotEmpty() && g.device.paths.values.none { it.isNotBlank() } && g.device.exe.isBlank()) {
                add(stringResource(R.string.edit_native))
            }
            addAll(elsewhere.map { it.name })
        }
        if (others.isNotEmpty()) {
            Hint(stringResource(R.string.edit_elsewhere, others.joinToString(", ")))
        }
        AppButton(onClick = { adding = true }, kind = BtnKind.Secondary, icon = Icons.Outlined.Add) {
            Text(stringResource(R.string.action_add_emulator))
        }
    }

    if (adding) {
        AddEmulatorDialog(g, state.emulators, onDismiss = { adding = false }) { emu, shareWith ->
            adding = false
            vm.addGameEmulator(g.id, emu.id, shareWith, emu.name)
        }
    }
    removing?.let { p ->
        // What the core would refuse, said up front: another emulator uses its
        // saves, or it is the game's only way of keeping them.
        val sharer = g.emulators.firstOrNull { it.shareWith == p.emulatorId }
        val last = g.slots.isEmpty() && g.emulators.size == 1
        val blocked = when {
            sharer != null -> stringResource(R.string.edit_remove_shared, sharer.name)
            last -> stringResource(R.string.edit_remove_last)
            else -> null
        }
        AlertDialog(
            onDismissRequest = { removing = null },
            title = { Text(stringResource(R.string.edit_remove_title, p.name, g.name)) },
            text = { Text(blocked ?: stringResource(R.string.edit_remove_text, p.name)) },
            confirmButton = {
                if (blocked != null) {
                    AppButton(onClick = { removing = null }) { Text(stringResource(R.string.action_ok)) }
                } else {
                    AppButton(onClick = {
                        removing = null
                        vm.removeGameEmulator(g.id, p.emulatorId, p.name)
                    }, kind = BtnKind.Danger) { Text(stringResource(R.string.action_remove)) }
                }
            },
            dismissButton = if (blocked != null) {
                null
            } else {
                { AppButton(onClick = { removing = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } }
            },
        )
    }
}

/**
 * Picks an emulator set up on this device that doesn't play the game yet, and
 * where its saves go: shared with an emulator of the same kind the game has
 * (suggested), or a folder of its own.
 */
@Composable
private fun AddEmulatorDialog(
    g: GameView,
    emulators: List<EmulatorView>,
    onDismiss: () -> Unit,
    onAdd: (EmulatorView, String) -> Unit,
) {
    val candidates = emulators.filter { e -> e.device.exe.isNotBlank() && g.emulators.none { it.emulatorId == e.id } }
    val byId = emulators.associateBy { it.id }
    var chosen by rememberSaveable { mutableStateOf(candidates.singleOrNull()?.id.orEmpty()) }
    val emu = candidates.firstOrNull { it.id == chosen }
    // Saves can be shared with an emulator of the same kind that keeps its own.
    val targets = if (emu == null || emu.preset.isEmpty()) {
        emptyList()
    } else {
        g.emulators.filter { it.shareWith.isEmpty() && byId[it.emulatorId]?.preset == emu.preset }
    }
    var shareWith by rememberSaveable(chosen) { mutableStateOf(targets.firstOrNull()?.emulatorId.orEmpty()) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.edit_add_title, g.name)) },
        text = {
            Column(Modifier.heightIn(max = 480.dp).verticalScroll(rememberScrollState())) {
                if (candidates.isEmpty()) {
                    Hint(stringResource(R.string.edit_add_none))
                }
                for (e in candidates) {
                    ChoiceRow(chosen == e.id, { chosen = e.id }) {
                        AppIcon(e.device.exe.substringBefore('/'), 32)
                        Text(e.name, style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))
                    }
                }
                if (emu != null && targets.isNotEmpty()) {
                    Text(
                        stringResource(R.string.edit_saves),
                        style = MaterialTheme.typography.labelLarge,
                        modifier = Modifier.padding(top = 16.dp, bottom = 4.dp),
                    )
                    for (t in targets) {
                        SavesChoice(
                            stringResource(R.string.edit_share_with, t.name),
                            stringResource(R.string.edit_share_hint),
                            shareWith == t.emulatorId,
                        ) { shareWith = t.emulatorId }
                    }
                    SavesChoice(
                        stringResource(R.string.edit_own_saves),
                        stringResource(R.string.edit_own_hint),
                        shareWith.isEmpty(),
                    ) { shareWith = "" }
                }
            }
        },
        confirmButton = {
            AppButton(onClick = { emu?.let { onAdd(it, shareWith) } }, enabled = emu != null) {
                Text(stringResource(R.string.action_add))
            }
        },
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
    )
}

@Composable
private fun SavesChoice(title: String, hint: String, selected: Boolean, onClick: () -> Unit) {
    ChoiceRow(selected, onClick) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyMedium)
            Hint(hint)
        }
    }
}
