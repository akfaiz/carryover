package com.akfaiz.carryover.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Restore
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Leftover
import com.akfaiz.carryover.core.TrashEmulator
import com.akfaiz.carryover.core.TrashGame

/**
 * What was deleted: games and emulators that can still be restored (or deleted
 * for good, with or without their cloud saves), and the cloud save folders no
 * game uses. It is how a user who only has Android frees the cloud again.
 */
@Composable
fun TrashScreen(nav: Nav, vm: TrashViewModel = viewModel()) {
    val state by vm.state.collectAsStateWithLifecycle()
    var forGame by remember { mutableStateOf<TrashGame?>(null) }
    var forEmulator by remember { mutableStateOf<TrashEmulator?>(null) }
    var forLeftover by remember { mutableStateOf<Leftover?>(null) }
    // The view model outlives the page: read the trash again each time it opens.
    LaunchedEffect(Unit) { vm.refresh() }

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { PageTopBar(stringResource(R.string.trash_title), nav::back) }) { padding ->
        PageColumn(padding) {
            val intro = stringResource(R.string.trash_intro, state.days)
            val empty = state.loaded && state.games.isEmpty() && state.emulators.isEmpty()
            if (empty) {
                EmptyState(Icons.Outlined.Delete, stringResource(R.string.trash_empty), intro)
            } else {
                Hint(intro)
            }

            if (state.games.isNotEmpty()) {
                SectionCard(stringResource(R.string.trash_games)) {
                    ListRows(state.games) { g ->
                        TrashItem(
                            name = g.name,
                            deletedAt = g.deletedAt,
                            expiresAt = g.expiresAt,
                            busy = g.id in state.busy,
                            onRestore = { vm.restoreGame(g) },
                            onDelete = { forGame = g },
                        )
                    }
                }
            }

            if (state.emulators.isNotEmpty()) {
                SectionCard(stringResource(R.string.trash_emulators)) {
                    ListRows(state.emulators) { e ->
                        TrashItem(
                            name = e.name,
                            deletedAt = e.deletedAt,
                            expiresAt = e.expiresAt,
                            busy = e.id in state.busy,
                            onRestore = { vm.restoreEmulator(e) },
                            onDelete = { forEmulator = e },
                        )
                    }
                }
            }

            SectionCard(stringResource(R.string.trash_leftovers)) {
                Hint(stringResource(R.string.trash_leftovers_hint))
                val leftovers = state.leftovers
                if (leftovers != null && leftovers.isEmpty()) {
                    Hint(stringResource(R.string.trash_leftovers_none))
                }
                if (!leftovers.isNullOrEmpty()) {
                    ListRows(leftovers) { l ->
                        LeftoverItem(l, busy = l.key in state.busy, onBringBack = { vm.restoreLeftover(l) }, onDelete = { forLeftover = l })
                    }
                }
                AppButton(onClick = vm::lookInCloud, kind = BtnKind.Secondary, enabled = !state.lookingInCloud) {
                    if (state.lookingInCloud) Spinner(small = true) else Text(stringResource(R.string.action_look_in_cloud))
                }
            }
        }
    }

    forGame?.let { g ->
        DeleteForeverDialog(
            name = g.name,
            onKeepSaves = {
                forGame = null
                vm.deleteGameForever(g, withSaves = false)
            },
            onWithSaves = {
                forGame = null
                vm.deleteGameForever(g, withSaves = true)
            },
            onDismiss = { forGame = null },
        )
    }
    forEmulator?.let { e ->
        DeleteForeverDialog(
            name = e.name,
            onKeepSaves = {
                forEmulator = null
                vm.deleteEmulatorForever(e, withSaves = false)
            },
            onWithSaves = {
                forEmulator = null
                vm.deleteEmulatorForever(e, withSaves = true)
            },
            onDismiss = { forEmulator = null },
        )
    }
    forLeftover?.let { l ->
        AlertDialog(
            onDismissRequest = { forLeftover = null },
            title = { Text(stringResource(R.string.dialog_leftover_delete_title, l.title)) },
            text = { Text(stringResource(R.string.dialog_leftover_delete_text)) },
            confirmButton = {
                AppButton(
                    onClick = {
                        forLeftover = null
                        vm.deleteLeftover(l)
                    },
                    kind = BtnKind.Danger,
                ) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { AppButton(onClick = { forLeftover = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/** A deleted game or emulator: when it was deleted, how long it can be restored, and what to do. */
@Composable
private fun ColumnScope.TrashItem(name: String, deletedAt: String, expiresAt: String, busy: Boolean, onRestore: () -> Unit, onDelete: () -> Unit) {
    val left = daysLeft(expiresAt)
    val leftText = when (left) {
        null -> ""
        0 -> stringResource(R.string.trash_expiring)
        else -> pluralStringResource(R.plurals.trash_days_left, left, left)
    }
    Text(name, style = MaterialTheme.typography.bodyLarge)
    Hint(stringResource(R.string.trash_item_info, relativeTime(deletedAt), leftText))
    Actions(busy) {
        AppButton(onClick = onRestore, kind = BtnKind.Secondary, icon = Icons.Outlined.Restore) { Text(stringResource(R.string.action_restore)) }
        AppButton(onClick = onDelete, kind = BtnKind.Danger) { Text(stringResource(R.string.action_delete_forever)) }
    }
}

/** A cloud save folder no game uses: what is in it, and what to do. */
@Composable
private fun ColumnScope.LeftoverItem(l: Leftover, busy: Boolean, onBringBack: () -> Unit, onDelete: () -> Unit) {
    Text(l.title, style = MaterialTheme.typography.bodyLarge)
    Hint(pluralStringResource(R.plurals.files_size, l.files, l.files, formatBytes(l.bytes)))
    Actions(busy) {
        AppButton(onClick = onBringBack, kind = BtnKind.Secondary, icon = Icons.Outlined.Restore) { Text(stringResource(R.string.action_bring_back)) }
        AppButton(onClick = onDelete, kind = BtnKind.Danger) { Text(stringResource(R.string.action_delete)) }
    }
}

/** The buttons of an item, or a spinner while its action runs. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Actions(busy: Boolean, buttons: @Composable () -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (busy) Spinner() else buttons()
    }
}

/**
 * Deleting for good: the settings can't come back, and the user decides what
 * happens to the saves in the cloud (the ones kept show up as leftovers).
 */
@Composable
private fun DeleteForeverDialog(name: String, onKeepSaves: () -> Unit, onWithSaves: () -> Unit, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.dialog_forever_title, name)) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(stringResource(R.string.dialog_forever_text))
                Hint(stringResource(R.string.dialog_forever_hint))
            }
        },
        // The three choices are one column in the confirm slot: with a column in the dismiss slot too, the
        // dialog spaces the two apart and cuts the last button off.
        confirmButton = {
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                AppButton(onClick = onWithSaves, kind = BtnKind.Danger) { Text(stringResource(R.string.action_delete_with_saves)) }
                AppButton(onClick = onKeepSaves, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_delete_keep_saves)) }
                AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) }
            }
        },
    )
}
