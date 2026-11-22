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
import androidx.compose.material3.Button
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
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Backup
import com.akfaiz.carryover.core.BackupKind

/**
 * The backups of one profile of a game: the copies Carryover keeps of its saves
 * (before a download or an upload replaces them, if cloud backups are on, and
 * the ones the user asks for), which can be pinned, deleted and restored.
 */
@Composable
fun BackupsScreen(key: String, state: UiState, nav: Nav, vm: BackupsViewModel = viewModel()) {
    val s by vm.state.collectAsStateWithLifecycle()
    var restoring by remember { mutableStateOf<Backup?>(null) }
    var deleting by remember { mutableStateOf<Backup?>(null) }
    // The view model outlives the page: it lists the backups of this profile each time the page opens.
    LaunchedEffect(key) { vm.open(key) }
    val game = state.games.firstOrNull { g -> g.profiles.any { it.key == key } }
    val profile = game?.profiles?.firstOrNull { it.key == key }

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { BackBar(stringResource(R.string.backups_title), nav) }) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (game != null) {
                Text(
                    listOfNotNull(game.name, profile?.emulator).joinToString(" · "),
                    style = MaterialTheme.typography.titleMedium,
                )
            }
            Text(
                if (s.kept > 0) {
                    pluralStringResource(R.plurals.backups_intro_on, s.kept, s.kept)
                } else {
                    stringResource(R.string.backups_intro_off)
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                Button(onClick = vm::backupNow, enabled = state.cloudReady && profile?.canSync == true && !s.busy) {
                    Text(stringResource(if (s.making) R.string.backups_making else R.string.action_backup_now))
                }
                if (s.busy) CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
                if (s.cancellable) TextButton(onClick = vm::cancel) { Text(stringResource(R.string.action_cancel)) }
            }
            if (profile != null && !profile.canSync) {
                Text(stringResource(R.string.backups_need_folders), style = MaterialTheme.typography.bodySmall)
            }

            when {
                !state.cloudReady -> Text(stringResource(R.string.backups_need_cloud), style = MaterialTheme.typography.bodyMedium)
                s.loadFailed && s.backups.isEmpty() -> {
                    Text(stringResource(R.string.backups_load_failed), style = MaterialTheme.typography.bodyMedium)
                    OutlinedButton(onClick = vm::refresh) { Text(stringResource(R.string.action_retry)) }
                }
                !s.loaded -> CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
                s.backups.isEmpty() -> Text(stringResource(R.string.backups_none), style = MaterialTheme.typography.bodyMedium)
                else -> for (b in s.backups) {
                    BackupItem(
                        b,
                        busy = s.acting == b.name,
                        enabled = !s.busy,
                        onRestore = { restoring = b },
                        onPin = { vm.togglePinned(b) },
                        onDelete = { deleting = b },
                    )
                }
            }
        }
    }

    restoring?.let { b ->
        AlertDialog(
            onDismissRequest = { restoring = null },
            title = { Text(stringResource(R.string.dialog_restore_backup_title)) },
            text = { Text(stringResource(R.string.dialog_restore_backup_text, relativeTime(b.time))) },
            confirmButton = {
                TextButton(
                    onClick = {
                        restoring = null
                        vm.restore(b)
                    },
                ) { Text(stringResource(R.string.action_restore)) }
            },
            dismissButton = { TextButton(onClick = { restoring = null }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
    deleting?.let { b ->
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text(stringResource(R.string.dialog_delete_backup_title)) },
            text = {
                val date = relativeTime(b.time)
                Text(
                    stringResource(
                        when {
                            b.local -> R.string.dialog_delete_backup_local_text
                            b.pinned -> R.string.dialog_delete_backup_pinned_text
                            else -> R.string.dialog_delete_backup_text
                        },
                        date,
                    ),
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        deleting = null
                        vm.delete(b)
                    },
                ) { Text(stringResource(R.string.action_delete), color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { deleting = null }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/** One backup: when, whose saves and why, and what to do with it. */
@Composable
private fun BackupItem(b: Backup, busy: Boolean, enabled: Boolean, onRestore: () -> Unit, onPin: () -> Unit, onDelete: () -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(relativeTime(b.time), style = MaterialTheme.typography.titleMedium)
            Text(
                backupDescription(b),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
                if (busy) {
                    CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
                } else {
                    OutlinedButton(onClick = onRestore, enabled = enabled) { Text(stringResource(R.string.action_restore)) }
                    // A copy a device keeps for itself is not shared, so there is nothing to protect from pruning.
                    if (!b.local) {
                        TextButton(onClick = onPin, enabled = enabled) {
                            Text(stringResource(if (b.pinned) R.string.action_unpin else R.string.action_pin))
                        }
                    }
                    TextButton(onClick = onDelete, enabled = enabled) {
                        Text(stringResource(R.string.action_delete), color = MaterialTheme.colorScheme.error)
                    }
                }
            }
        }
    }
}

/** Whose saves a backup holds and why it was made ("Pixel's saves (manual backup) · pinned"). */
@Composable
private fun backupDescription(b: Backup): String {
    val what = stringResource(
        when {
            b.local -> R.string.backup_kind_safety
            b.kind == BackupKind.Local -> R.string.backup_kind_local
            b.kind == BackupKind.Manual -> R.string.backup_kind_manual
            else -> R.string.backup_kind_cloud
        },
        b.deviceName,
    )
    return if (b.pinned) stringResource(R.string.backup_pinned, what) else what
}
