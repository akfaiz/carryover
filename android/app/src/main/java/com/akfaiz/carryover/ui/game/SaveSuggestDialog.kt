package com.akfaiz.carryover.ui.game

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SafPath
import com.akfaiz.carryover.core.SaveKind
import com.akfaiz.carryover.core.SaveSuggestion
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Callout
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.Pill
import com.akfaiz.carryover.ui.kit.Spinner
import com.akfaiz.carryover.ui.kit.SubCard
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.state.suggestionFilters

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
                        Spinner(small = true)
                        Text(stringResource(R.string.suggest_looking), style = MaterialTheme.typography.bodyMedium)
                    }
                    state.error != null -> Callout(Tone.Err, Icons.Outlined.ErrorOutline, state.error)
                    result != null -> {
                        if (result.note.isNotEmpty()) Hint(result.note)
                        if (result.suggestions.isEmpty()) {
                            Text(stringResource(R.string.suggest_none), style = MaterialTheme.typography.bodyMedium)
                        } else {
                            Hint(stringResource(R.string.suggest_hint))
                        }
                        for (sg in result.suggestions) SuggestionItem(sg, enabled = !state.applying, onUse = { onUse(sg) })
                    }
                }
            }
        },
        confirmButton = { AppButton(onClick = onClose, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_close)) } },
    )
}

/** One suggestion: the game it is for, the folder (and what is taken from it), and whether it is there. */
@Composable
private fun SuggestionItem(sg: SaveSuggestion, enabled: Boolean, onUse: () -> Unit) {
    val filters = suggestionFilters(sg.include)
    SubCard {
        Text(sg.game, style = MaterialTheme.typography.titleSmall)
        when (sg.kind) {
            SaveKind.Account -> Pill(stringResource(R.string.suggest_kind_account), Tone.Neutral)
            SaveKind.Device -> Pill(stringResource(R.string.suggest_kind_device), Tone.Neutral)
            SaveKind.None -> Unit
        }
        Hint(SafPath.describe(sg.path))
        if (filters.isNotEmpty()) Hint(stringResource(R.string.suggest_filters, filters.joinToString(", ")))
        Row(Modifier.padding(top = 6.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            AppButton(onClick = onUse, kind = BtnKind.Secondary, enabled = enabled) { Text(stringResource(R.string.action_use)) }
            if (sg.exists) Pill(stringResource(R.string.suggest_found), Tone.Ok)
        }
    }
}
