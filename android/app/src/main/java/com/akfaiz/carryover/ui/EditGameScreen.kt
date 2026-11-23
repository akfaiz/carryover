package com.akfaiz.carryover.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
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
import com.akfaiz.carryover.core.SaveSlot

/**
 * Edits a game for every device: its name, and the emulators it is played
 * with. An emulator added here may share the saves of one the game already
 * has (Lemon next to Eden: the same Switch saves), so both sync one cloud
 * folder. Emulators used only on other devices (Ryujinx, the PC version) are
 * listed but left alone.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditGameScreen(id: String, state: UiState, vm: AppViewModel, nav: Nav) {
    // The whole game, reloaded with the library (the library's copy leaves out other devices' emulators).
    val g by produceState<GameView?>(null, id, state.games) { value = vm.fullGame(id) }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.edit_game)) },
                navigationIcon = {
                    IconButton(onClick = nav::back) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back))
                    }
                },
                windowInsets = pageBarInsets(),
            )
        },
    ) { padding ->
        val game = g ?: return@Scaffold
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(
                Modifier.widthIn(max = 760.dp).fillMaxWidth().padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                NameCard(game, vm)
                EmulatorsCard(game, state, vm)
                SlotsCards(game, state, vm)
            }
        }
    }
}

@Composable
private fun NameCard(g: GameView, vm: AppViewModel) {
    var name by remember(g.id, g.name) { mutableStateOf(g.name) }
    SectionCard(stringResource(R.string.field_name)) {
        OutlinedTextField(name, { name = it }, singleLine = true, modifier = Modifier.fillMaxWidth().dpadExit())
        Button(onClick = { vm.renameGame(g.id, name) }, enabled = name.isNotBlank() && name.trim() != g.name) {
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
        for (p in here) {
            val exe = byId[p.emulatorId]?.device?.exe.orEmpty()
            SettingRow(
                icon = null,
                title = p.name,
                detail = when {
                    p.shareWith.isNotEmpty() -> stringResource(R.string.edit_shares, nameOf(p.shareWith))
                    exe.isNotBlank() -> appLabel(ctx, exe)
                    else -> stringResource(R.string.emulator_not_set_up)
                },
                leading = { AppIcon(exe.substringBefore('/'), 36) },
                trailing = {
                    IconButton(onClick = { removing = p }) {
                        Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.action_remove))
                    }
                },
            )
        }
        // The game's own (PC) saves and emulators without an Android app stay on the other devices.
        val others = buildList {
            if (g.slots.isNotEmpty() && g.device.paths.values.none { it.isNotBlank() } && g.device.exe.isBlank()) {
                add(stringResource(R.string.edit_native))
            }
            addAll(elsewhere.map { it.name })
        }
        if (others.isNotEmpty()) {
            Text(
                stringResource(R.string.edit_elsewhere, others.joinToString(", ")),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
        }
        OutlinedButton(onClick = { adding = true }) {
            Icon(Icons.Filled.Add, null)
            Text(stringResource(R.string.action_add_emulator), Modifier.padding(start = 8.dp))
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
                    TextButton(onClick = { removing = null }) { Text(stringResource(R.string.action_ok)) }
                } else {
                    TextButton(onClick = {
                        removing = null
                        vm.removeGameEmulator(g.id, p.emulatorId, p.name)
                    }) { Text(stringResource(R.string.action_remove)) }
                }
            },
            dismissButton = if (blocked != null) {
                null
            } else {
                { TextButton(onClick = { removing = null }) { Text(stringResource(R.string.action_cancel)) } }
            },
        )
    }
}

/** One way a game's saves are kept, as the slot editor sees it: [shareWith] is the emulator whose saves it uses. */
private class SlotProfile(val key: String, val name: String?, val slots: List<SaveSlot>, val shareWith: String)

/**
 * The save locations of each profile that plays here, such as a game's saves and
 * its save states: add one, rename one, or remove one. They are the game's on
 * every device (each device chooses its own folder for them on the game's
 * page); a removed one only stops syncing, its cloud files stay. A profile that
 * uses another emulator's saves has no locations of its own to change.
 */
@Composable
private fun SlotsCards(g: GameView, state: UiState, vm: AppViewModel) {
    val byId = state.emulators.associateBy { it.id }
    val own = g.slots.isNotEmpty() && (g.hasDevice || g.device.paths.values.any { it.isNotBlank() } || g.device.exe.isNotBlank())
    val profiles = buildList {
        if (own) add(SlotProfile(g.id, null, g.slots, ""))
        for (p in g.emulators) {
            if (byId[p.emulatorId]?.let(EmulatorPresets::onAndroid) == true) add(SlotProfile(p.key, p.name, p.slots, p.shareWith))
        }
    }
    val names = g.emulators.associate { it.emulatorId to it.name }
    for (p in profiles) {
        SlotsCard(p, names[p.shareWith] ?: p.shareWith, vm)
    }
}

@Composable
private fun SlotsCard(p: SlotProfile, sharedName: String, vm: AppViewModel) {
    // The slot being renamed, "" for a new one; null while no dialog is open.
    var naming by remember { mutableStateOf<String?>(null) }
    var removing by remember { mutableStateOf<SaveSlot?>(null) }
    val title = p.name?.let { stringResource(R.string.edit_slots_for, it) } ?: stringResource(R.string.edit_slots_title)

    SectionCard(title) {
        if (p.shareWith.isNotEmpty()) {
            Text(
                stringResource(R.string.edit_slots_shared, p.name.orEmpty(), sharedName),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
            return@SectionCard
        }
        for (sl in p.slots) {
            SettingRow(
                icon = null,
                title = sl.title,
                detail = filtersSummary(sl),
                onClick = { naming = sl.key },
                // The last one stays: a profile with no location has nothing to sync.
                trailing = if (p.slots.size > 1) {
                    {
                        IconButton(onClick = { removing = sl }) {
                            Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.action_remove))
                        }
                    }
                } else {
                    null
                },
            )
        }
        OutlinedButton(onClick = { naming = "" }) {
            Icon(Icons.Filled.Add, null)
            Text(stringResource(R.string.action_add_slot), Modifier.padding(start = 8.dp))
        }
    }

    naming?.let { key ->
        val current = p.slots.firstOrNull { it.key == key }
        SlotDialog(
            title = stringResource(if (current == null) R.string.edit_slot_new else R.string.edit_slot_rename),
            initial = current,
            onDismiss = { naming = null },
        ) { edited ->
            naming = null
            // Existing slots go back with their keys, a new one with no key.
            val next = if (current == null) p.slots + edited else p.slots.map { if (it.key == key) edited else it }
            vm.setSlots(p.key, next)
        }
    }
    removing?.let { sl ->
        AlertDialog(
            onDismissRequest = { removing = null },
            title = { Text(stringResource(R.string.edit_slot_remove_title, sl.title)) },
            text = { Text(stringResource(R.string.edit_slot_remove_text, sl.title)) },
            confirmButton = {
                TextButton(onClick = {
                    removing = null
                    vm.setSlots(p.key, p.slots.filter { it.key != sl.key })
                }) { Text(stringResource(R.string.action_remove)) }
            },
            dismissButton = { TextButton(onClick = { removing = null }) { Text(stringResource(R.string.action_cancel)) } },
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
                    Text(stringResource(R.string.edit_add_none), style = MaterialTheme.typography.bodyMedium)
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
            TextButton(onClick = { emu?.let { onAdd(it, shareWith) } }, enabled = emu != null) {
                Text(stringResource(R.string.action_add))
            }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

@Composable
private fun SavesChoice(title: String, hint: String, selected: Boolean, onClick: () -> Unit) {
    ChoiceRow(selected, onClick) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyMedium)
            Text(hint, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
