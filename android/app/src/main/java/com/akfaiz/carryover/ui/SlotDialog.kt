package com.akfaiz.carryover.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SaveSlot

/**
 * A save location's filters as one line ("Only *.sav · Except *.log"), or null
 * when it takes every file in its folder.
 */
@Composable
fun filtersSummary(slot: SaveSlot): String? {
    val include = shownFilters(slot.include)
    val exclude = shownFilters(slot.exclude)
    val parts = buildList {
        if (include.isNotEmpty()) add(stringResource(R.string.slot_filters_only, include.joinToString(", ")))
        if (exclude.isNotEmpty()) add(stringResource(R.string.slot_filters_except, exclude.joinToString(", ")))
    }
    return parts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
}

/**
 * Edits a save location: its name and which files of its folder sync (the
 * patterns to include, one per line, none = all; and those to leave out).
 * [initial] null adds a new one. [onSave] gets the location as edited, with
 * the key of [initial] (a new one has none: the core makes it from the name).
 */
@Composable
fun SlotDialog(title: String, initial: SaveSlot?, onDismiss: () -> Unit, onSave: (SaveSlot) -> Unit) {
    val startName = initial?.label?.ifBlank { initial.key }.orEmpty()
    val startInclude = filtersText(initial?.include.orEmpty())
    val startExclude = filtersText(initial?.exclude.orEmpty())
    var name by rememberSaveable { mutableStateOf(startName) }
    var include by rememberSaveable { mutableStateOf(startInclude) }
    var exclude by rememberSaveable { mutableStateOf(startExclude) }
    val changed = name.trim() != startName ||
        parseFilters(include) != parseFilters(startInclude) ||
        parseFilters(exclude) != parseFilters(startExclude)
    val mono = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace)
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    name,
                    { name = it },
                    singleLine = true,
                    label = { Text(stringResource(R.string.field_name)) },
                    modifier = Modifier.fillMaxWidth().dpadExit(),
                )
                Text(
                    stringResource(R.string.edit_slot_hint),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedTextField(
                    include,
                    { include = it },
                    minLines = 2,
                    textStyle = mono,
                    label = { Text(stringResource(R.string.field_include)) },
                    placeholder = { Text(stringResource(R.string.slot_include_example), style = mono) },
                    modifier = Modifier.fillMaxWidth().dpadExit(),
                )
                OutlinedTextField(
                    exclude,
                    { exclude = it },
                    minLines = 2,
                    textStyle = mono,
                    label = { Text(stringResource(R.string.field_exclude)) },
                    placeholder = { Text(stringResource(R.string.slot_exclude_example), style = mono) },
                    modifier = Modifier.fillMaxWidth().dpadExit(),
                )
                Text(
                    stringResource(R.string.edit_slot_filters_hint),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        },
        confirmButton = {
            AppButton(
                onClick = {
                    val edited = (initial ?: SaveSlot()).copy(
                        label = name.trim(),
                        include = parseFilters(include),
                        exclude = parseFilters(exclude),
                    )
                    onSave(edited)
                },
                enabled = name.isNotBlank() && changed,
            ) { Text(stringResource(R.string.action_save)) }
        },
        dismissButton = { AppButton(onClick = onDismiss, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
    )
}
