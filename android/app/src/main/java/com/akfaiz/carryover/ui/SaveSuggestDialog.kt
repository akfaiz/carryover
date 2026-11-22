package com.akfaiz.carryover.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SafPath
import com.akfaiz.carryover.core.SaveKind
import com.akfaiz.carryover.core.SaveSuggestion

/**
 * The save folders suggested for a game's emulator profile (see
 * [SuggestViewModel]): where the emulator keeps this game's saves, and which
 * of those are on this device. Tapping Use makes one the profile's folder.
 */
@Composable
fun SaveSuggestDialog(state: SuggestState, onUse: (SaveSuggestion) -> Unit, onClose: () -> Unit) {
    val result = state.result
    AlertDialog(
        onDismissRequest = onClose,
        title = { Text(stringResource(R.string.suggest_title)) },
        text = {
            Column(
                Modifier.heightIn(max = 460.dp).verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                when {
                    state.loading -> Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                        Text(stringResource(R.string.suggest_looking), style = MaterialTheme.typography.bodyMedium)
                    }
                    state.error != null -> Text(state.error, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
                    result != null -> {
                        if (result.note.isNotEmpty()) Text(result.note, style = MaterialTheme.typography.bodySmall)
                        if (result.suggestions.isEmpty()) {
                            Text(stringResource(R.string.suggest_none), style = MaterialTheme.typography.bodyMedium)
                        } else {
                            Text(stringResource(R.string.suggest_hint), style = MaterialTheme.typography.bodySmall)
                        }
                        for (sg in result.suggestions) SuggestionItem(sg, enabled = !state.applying, onUse = { onUse(sg) })
                    }
                }
            }
        },
        confirmButton = { TextButton(onClick = onClose) { Text(stringResource(R.string.action_close)) } },
    )
}

/** One suggestion: the game it is for, the folder (and what is taken from it), and whether it is there. */
@Composable
private fun SuggestionItem(sg: SaveSuggestion, enabled: Boolean, onUse: () -> Unit) {
    val filters = suggestionFilters(sg.include)
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(sg.game, style = MaterialTheme.typography.titleSmall)
            when (sg.kind) {
                SaveKind.Account -> Text(stringResource(R.string.suggest_kind_account), style = MaterialTheme.typography.labelMedium)
                SaveKind.Device -> Text(stringResource(R.string.suggest_kind_device), style = MaterialTheme.typography.labelMedium)
                SaveKind.None -> Unit
            }
            Text(SafPath.describe(sg.path), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (filters.isNotEmpty()) {
                Text(
                    stringResource(R.string.suggest_filters, filters.joinToString(", ")),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Row(Modifier.padding(top = 6.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedButton(onClick = onUse, enabled = enabled) { Text(stringResource(R.string.action_use)) }
                if (sg.exists) {
                    Text(stringResource(R.string.suggest_found), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                }
            }
        }
    }
}
