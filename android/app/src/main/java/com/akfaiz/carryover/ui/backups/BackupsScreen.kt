package com.akfaiz.carryover.ui.backups

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.PushPin
import androidx.compose.material.icons.outlined.Restore
import androidx.compose.material.icons.outlined.WarningAmber
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
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Backup
import com.akfaiz.carryover.core.BackupKind
import com.akfaiz.carryover.ui.components.relativeTime
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Callout
import com.akfaiz.carryover.ui.kit.EmptyState
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.ListRows
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.Spinner
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.state.UiState

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

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { PageTopBar(stringResource(R.string.backups_title), nav::back) }) { padding ->
        PageColumn(padding) {
            if (game != null) {
                Text(
                    listOfNotNull(game.name, profile?.emulator).joinToString(" · "),
                    style = MaterialTheme.typography.titleMedium,
                )
            }
            Hint(
                if (s.kept > 0) {
                    pluralStringResource(R.plurals.backups_intro_on, s.kept, s.kept)
                } else {
                    stringResource(R.string.backups_intro_off)
                },
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                AppButton(
                    onClick = vm::backupNow,
                    icon = Icons.Outlined.CloudUpload,
                    enabled = state.cloudReady && profile?.canSync == true && !s.busy,
                ) {
                    Text(stringResource(if (s.making) R.string.backups_making else R.string.action_backup_now))
                }
                if (s.busy) Spinner()
                if (s.cancellable) AppButton(onClick = vm::cancel, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) }
            }
            if (profile != null && !profile.canSync) {
                Callout(Tone.Warn, Icons.Outlined.WarningAmber, stringResource(R.string.backups_need_folders))
            }

            when {
                !state.cloudReady -> Callout(Tone.Info, Icons.Outlined.Info, stringResource(R.string.backups_need_cloud))
                s.loadFailed && s.backups.isEmpty() -> Callout(
                    Tone.Err,
                    Icons.Outlined.ErrorOutline,
                    stringResource(R.string.backups_load_failed),
                    actions = {
                        AppButton(onClick = vm::refresh, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_retry)) }
                    },
                )
                !s.loaded -> Spinner()
                s.backups.isEmpty() -> EmptyState(Icons.Outlined.History, stringResource(R.string.backups_none), stringResource(R.string.backups_none_text))
                else -> SectionCard(null) {
                    ListRows(s.backups) { b ->
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
    }

    restoring?.let { b ->
        AlertDialog(
            onDismissRequest = { restoring = null },
            title = { Text(stringResource(R.string.dialog_restore_backup_title)) },
            text = { Text(stringResource(R.string.dialog_restore_backup_text, relativeTime(b.time))) },
            confirmButton = {
                AppButton(
                    onClick = {
                        restoring = null
                        vm.restore(b)
                    },
                ) { Text(stringResource(R.string.action_restore)) }
            },
            dismissButton = { AppButton(onClick = { restoring = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
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
                AppButton(
                    onClick = {
                        deleting = null
                        vm.delete(b)
                    },
                    kind = BtnKind.Danger,
                ) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { AppButton(onClick = { deleting = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/** One backup: when, whose saves and why, and what to do with it. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ColumnScope.BackupItem(b: Backup, busy: Boolean, enabled: Boolean, onRestore: () -> Unit, onPin: () -> Unit, onDelete: () -> Unit) {
    Text(relativeTime(b.time), style = MaterialTheme.typography.bodyLarge)
    Hint(backupDescription(b))
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (busy) {
            Spinner()
        } else {
            AppButton(onClick = onRestore, kind = BtnKind.Secondary, icon = Icons.Outlined.Restore, enabled = enabled) {
                Text(stringResource(R.string.action_restore))
            }
            // A copy a device keeps for itself is not shared, so there is nothing to protect from pruning.
            if (!b.local) {
                AppButton(onClick = onPin, kind = BtnKind.Ghost, icon = Icons.Outlined.PushPin, enabled = enabled) {
                    Text(stringResource(if (b.pinned) R.string.action_unpin else R.string.action_pin))
                }
            }
            AppButton(onClick = onDelete, kind = BtnKind.Danger, enabled = enabled) { Text(stringResource(R.string.action_delete)) }
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
