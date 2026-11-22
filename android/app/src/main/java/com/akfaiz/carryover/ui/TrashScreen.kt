package com.akfaiz.carryover.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
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

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { BackBar(stringResource(R.string.trash_title), nav) }) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(stringResource(R.string.trash_intro, state.days), style = MaterialTheme.typography.bodyMedium)

            Text(stringResource(R.string.trash_games), style = MaterialTheme.typography.titleSmall)
            if (state.games.isEmpty()) Nothing(state.loaded)
            for (g in state.games) {
                TrashItem(
                    name = g.name,
                    deletedAt = g.deletedAt,
                    expiresAt = g.expiresAt,
                    busy = g.id in state.busy,
                    onRestore = { vm.restoreGame(g) },
                    onDelete = { forGame = g },
                )
            }

            Text(stringResource(R.string.trash_emulators), style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(top = 8.dp))
            if (state.emulators.isEmpty()) Nothing(state.loaded)
            for (e in state.emulators) {
                TrashItem(
                    name = e.name,
                    deletedAt = e.deletedAt,
                    expiresAt = e.expiresAt,
                    busy = e.id in state.busy,
                    onRestore = { vm.restoreEmulator(e) },
                    onDelete = { forEmulator = e },
                )
            }

            Text(stringResource(R.string.trash_leftovers), style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(top = 8.dp))
            Text(
                stringResource(R.string.trash_leftovers_hint),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val leftovers = state.leftovers
            if (leftovers != null && leftovers.isEmpty()) {
                Text(stringResource(R.string.trash_leftovers_none), style = MaterialTheme.typography.bodySmall)
            }
            for (l in leftovers.orEmpty()) {
                LeftoverItem(l, busy = l.key in state.busy, onBringBack = { vm.restoreLeftover(l) }, onDelete = { forLeftover = l })
            }
            OutlinedButton(onClick = vm::lookInCloud, enabled = !state.lookingInCloud) {
                if (state.lookingInCloud) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                } else {
                    Text(stringResource(R.string.action_look_in_cloud))
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
                TextButton(
                    onClick = {
                        forLeftover = null
                        vm.deleteLeftover(l)
                    },
                ) { Text(stringResource(R.string.action_delete), color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { forLeftover = null }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

@Composable
private fun Nothing(loaded: Boolean) {
    if (loaded) Text(stringResource(R.string.trash_empty), style = MaterialTheme.typography.bodySmall)
}

/** A deleted game or emulator: when it was deleted, how long it can be restored, and what to do. */
@Composable
private fun TrashItem(name: String, deletedAt: String, expiresAt: String, busy: Boolean, onRestore: () -> Unit, onDelete: () -> Unit) {
    val left = daysLeft(expiresAt)
    val leftText = when (left) {
        null -> ""
        0 -> stringResource(R.string.trash_expiring)
        else -> pluralStringResource(R.plurals.trash_days_left, left, left)
    }
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(name, style = MaterialTheme.typography.titleMedium)
            Text(
                stringResource(R.string.trash_item_info, relativeTime(deletedAt), leftText),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Actions(busy) {
                OutlinedButton(onClick = onRestore) { Text(stringResource(R.string.action_restore)) }
                TextButton(onClick = onDelete) { Text(stringResource(R.string.action_delete_forever), color = MaterialTheme.colorScheme.error) }
            }
        }
    }
}

/** A cloud save folder no game uses: what is in it, and what to do. */
@Composable
private fun LeftoverItem(l: Leftover, busy: Boolean, onBringBack: () -> Unit, onDelete: () -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(l.title, style = MaterialTheme.typography.titleMedium)
            Text(
                pluralStringResource(R.plurals.files_size, l.files, l.files, formatBytes(l.bytes)),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Actions(busy) {
                OutlinedButton(onClick = onBringBack) { Text(stringResource(R.string.action_bring_back)) }
                TextButton(onClick = onDelete) { Text(stringResource(R.string.action_delete), color = MaterialTheme.colorScheme.error) }
            }
        }
    }
}

/** The buttons of an item, or a spinner while its action runs. */
@Composable
private fun Actions(busy: Boolean, buttons: @Composable () -> Unit) {
    Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
        if (busy) CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp) else buttons()
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
                Text(stringResource(R.string.dialog_forever_hint), style = MaterialTheme.typography.bodySmall)
            }
        },
        // The three choices are one column in the confirm slot: with a column in the dismiss slot too, the
        // dialog spaces the two apart and cuts the last button off.
        confirmButton = {
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                TextButton(onClick = onWithSaves) { Text(stringResource(R.string.action_delete_with_saves), color = MaterialTheme.colorScheme.error) }
                TextButton(onClick = onKeepSaves) { Text(stringResource(R.string.action_delete_keep_saves)) }
                TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) }
            }
        },
    )
}
