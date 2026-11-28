package com.akfaiz.carryover.ui.components

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Answer
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.PromptKind
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind

/** A question from a play session (before or after playing); the session waits for the answer. */
@Composable
fun PromptDialog(p: Prompt, onAnswer: (String) -> Unit) {
    when (p.kind) {
        PromptKind.Conflict -> AlertDialog(
            onDismissRequest = {},
            title = { Text(stringResource(R.string.prompt_conflict_title, p.gameName)) },
            text = {
                Column {
                    Text(stringResource(if (p.beforePlay) R.string.prompt_conflict_play else R.string.prompt_conflict_keep))
                    p.conflict?.let {
                        Text(stringResource(R.string.conflict_this_device, describe(it.local)), Modifier.padding(top = 12.dp))
                        Text(stringResource(R.string.conflict_cloud, describe(it.remote)), Modifier.padding(top = 4.dp))
                    }
                }
            },
            confirmButton = {
                Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                    AppButton(onClick = { onAnswer(Answer.Local.wire) }) { Text(stringResource(R.string.action_keep_local)) }
                    AppButton(onClick = { onAnswer(Answer.Cloud.wire) }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_keep_cloud)) }
                    // Skip leaves both sides as they are: before playing, the game starts with this device's saves.
                    AppButton(onClick = { onAnswer(Answer.Skip.wire) }, kind = BtnKind.Ghost) {
                        Text(stringResource(if (p.beforePlay) R.string.action_play_no_sync else R.string.action_decide_later))
                    }
                }
            },
        )
        PromptKind.Lock -> AlertDialog(
            onDismissRequest = {},
            title = {
                Text(
                    stringResource(
                        R.string.prompt_lock_title,
                        p.gameName,
                        p.lock?.deviceName?.ifEmpty { null } ?: stringResource(R.string.prompt_lock_another),
                    ),
                )
            },
            text = { Text(stringResource(R.string.prompt_lock_text)) },
            confirmButton = { AppButton(onClick = { onAnswer(Answer.Yes.wire) }) { Text(stringResource(R.string.action_play_anyway)) } },
            dismissButton = { AppButton(onClick = { onAnswer(Answer.No.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_dont_play)) } },
        )
        PromptKind.SyncError -> AlertDialog(
            onDismissRequest = {},
            title = { Text(stringResource(R.string.prompt_sync_error_title, p.gameName)) },
            text = {
                Column {
                    Text(coreText(p.error, p.errorMsg))
                    if (p.beforePlay) {
                        Spacer(Modifier.height(12.dp))
                        Text(stringResource(R.string.prompt_sync_error_play))
                    }
                }
            },
            confirmButton = {
                AppButton(onClick = { onAnswer(Answer.Yes.wire) }) {
                    Text(stringResource(if (p.beforePlay) R.string.action_play_anyway else R.string.action_ok))
                }
            },
            dismissButton = if (p.beforePlay) {
                { AppButton(onClick = { onAnswer(Answer.No.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_dont_play)) } }
            } else {
                null
            },
        )
        PromptKind.Emulator -> AlertDialog(
            onDismissRequest = { onAnswer(Answer.Cancel.wire) },
            title = { Text(stringResource(R.string.prompt_emulator_title, p.gameName)) },
            text = {
                Column {
                    for (c in p.choices) {
                        AppButton(onClick = { onAnswer(c.id) }, Modifier.fillMaxWidth(), BtnKind.Secondary) { Text(coreText(c.name, c.nameMsg)) }
                    }
                }
            },
            confirmButton = {},
            dismissButton = { AppButton(onClick = { onAnswer(Answer.Cancel.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
        PromptKind.Unknown -> onAnswer(Answer.No.wire)
    }
}
