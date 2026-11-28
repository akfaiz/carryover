package com.akfaiz.carryover.ui.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Autorenew
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.NewReleases
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.SystemUpdate
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.UpdateState
import com.akfaiz.carryover.core.UpdateStatus
import com.akfaiz.carryover.data.SettingsState
import com.akfaiz.carryover.ui.components.SectionHeader
import com.akfaiz.carryover.ui.components.SettingRow
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Hint

/** The settings page's updates: the channel, automatic checks, and the newer version when there is one. */
@Composable
fun UpdateSection(state: SettingsState, vm: SettingsViewModel) {
    val update by vm.update.collectAsStateWithLifecycle()
    var dialog by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadUpdate() }

    SectionHeader(stringResource(R.string.section_updates))
    Group {
        SettingRow(
            Icons.Outlined.NewReleases,
            stringResource(R.string.setting_update_channel),
            stringResource(
                if (state.updateChannel.isEmpty()) {
                    R.string.setting_update_channel_stable
                } else {
                    R.string.setting_update_channel_prerelease
                },
            ),
            detailLines = Int.MAX_VALUE,
            trailing = null,
        )
        SegmentedChoice(
            listOf("" to R.string.channel_stable, "prerelease" to R.string.channel_prerelease),
            state.updateChannel,
            vm::setUpdateChannel,
        )
        SettingRow(
            Icons.Outlined.Autorenew,
            stringResource(R.string.setting_update_auto),
            stringResource(R.string.setting_update_auto_hint),
            detailLines = Int.MAX_VALUE,
            onClick = { vm.setCheckUpdates(!state.checksUpdates) },
            trailing = { Switch(checked = state.checksUpdates, onCheckedChange = vm::setCheckUpdates) },
        )
        when {
            !update.supported -> SettingRow(Icons.Outlined.Info, stringResource(R.string.update_dev_build), trailing = null)
            update.newer -> SettingRow(
                Icons.Outlined.SystemUpdate,
                stringResource(R.string.update_available, update.latest),
                update.error.ifEmpty { stringResource(R.string.update_available_hint, update.current) },
                onClick = { dialog = true },
            )
            else -> {
                val checking = update.status == UpdateStatus.Checking
                SettingRow(
                    Icons.Outlined.Refresh,
                    stringResource(if (checking) R.string.update_checking else R.string.update_check),
                    update.error.ifEmpty {
                        if (update.status == UpdateStatus.Current) stringResource(R.string.update_current) else null
                    },
                    onClick = if (checking) null else vm::checkForUpdate,
                    trailing = null,
                )
            }
        }
    }
    if (dialog && update.newer) {
        UpdateDialog(
            update,
            onInstall = vm::installUpdate,
            onCancel = vm::cancelUpdate,
            onPage = { vm.openBrowser(update.url) },
            onDismiss = { dialog = false },
        )
    }
}

/** What is in the newer version, and the button that downloads and installs it. */
@Composable
private fun UpdateDialog(
    update: UpdateState,
    onInstall: () -> Unit,
    onCancel: () -> Unit,
    onPage: () -> Unit,
    onDismiss: () -> Unit,
) {
    val downloading = update.status == UpdateStatus.Downloading
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.update_title)) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(stringResource(R.string.update_available, update.latest))
                if (update.error.isNotEmpty()) Text(update.error, color = MaterialTheme.colorScheme.error)
                if (!update.installable) Hint(stringResource(R.string.update_not_installable))
                if (downloading) {
                    Hint(stringResource(R.string.update_downloading))
                    if (update.total > 0) {
                        LinearProgressIndicator(progress = { update.done.toFloat() / update.total }, modifier = Modifier.fillMaxWidth())
                    } else {
                        LinearProgressIndicator(Modifier.fillMaxWidth())
                    }
                }
                if (update.notes.isNotEmpty()) Text(update.notes, style = MaterialTheme.typography.bodySmall)
                if (update.url.isNotEmpty()) {
                    AppButton(onClick = onPage, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_release_page)) }
                }
            }
        },
        confirmButton = {
            if (downloading) {
                AppButton(onClick = onCancel, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_cancel)) }
            } else if (update.installable) {
                AppButton(onClick = onInstall) { Text(stringResource(R.string.action_update_install)) }
            }
        },
        dismissButton = {
            AppButton(onClick = onDismiss, kind = BtnKind.Secondary) {
                Text(stringResource(if (update.installable) R.string.action_later else R.string.action_close))
            }
        },
    )
}
