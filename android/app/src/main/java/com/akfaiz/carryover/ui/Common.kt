package com.akfaiz.carryover.ui

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Keyboard
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SafPath
import com.akfaiz.carryover.core.Storage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * A game file with a Browse button: the system file picker gives a content://
 * URI, which this app keeps permission to read, so it can let the emulator
 * read it when the game starts. A typed path works too. A picked file shows by
 * its name (with a button that clears it); [hint], or while it is empty the
 * default one, is shown under it.
 */
@Composable
fun GameFileField(value: String, onChange: (String) -> Unit, label: String, hint: String? = null) {
    val ctx = LocalContext.current
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        runCatching {
            ctx.contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        onChange(uri.toString())
    }
    val browse = @Composable {
        TextButton(onClick = { picker.launch(arrayOf("*/*")) }) { Text(stringResource(R.string.action_browse)) }
    }
    val supporting = hint ?: if (value.isBlank()) stringResource(R.string.game_file_hint) else null
    if (isContentUri(value)) {
        OutlinedTextField(
            value = rememberDocumentName(value),
            onValueChange = {},
            readOnly = true,
            label = { Text(label) },
            singleLine = true,
            supportingText = supporting?.let { { Text(it) } },
            trailingIcon = {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    IconButton(onClick = { onChange("") }) {
                        Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.action_clear))
                    }
                    browse()
                }
            },
            modifier = Modifier.fillMaxWidth().dpadExit(),
        )
        return
    }
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        singleLine = true,
        supportingText = supporting?.let { { Text(it) } },
        trailingIcon = browse,
        modifier = Modifier.fillMaxWidth().dpadExit(),
    )
}

/** A picked document's name, from its provider (on a background thread); until then, or failing that, from its URI. */
@Composable
fun rememberDocumentName(uri: String): String {
    val ctx = LocalContext.current
    val fallback = documentNameOf(uri)
    val name by produceState(fallback, uri) {
        value = withContext(Dispatchers.IO) {
            runCatching {
                ctx.contentResolver.query(Uri.parse(uri), arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
                    if (c.moveToFirst()) c.getString(0) else null
                }
            }.getOrNull()
        } ?: fallback
    }
    return name
}

/**
 * A folder path with a Browse button. The system picker returns a document
 * URI: a folder on device storage is kept as its path when Carryover may read
 * every file, any other through the Storage Access Framework (shown by where
 * it is, not editable; the keyboard button under it switches to typing a path).
 * [supporting] (or an error) is shown under the field; [note] and [actions]
 * share one line below it, so a field with extras takes one row more, not
 * several.
 */
@Composable
fun FolderField(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    supporting: String? = null,
    isError: Boolean = false,
    note: String? = null,
    actions: (@Composable RowScope.() -> Unit)? = null,
) {
    val ctx = LocalContext.current
    var pickError by remember { mutableStateOf<String?>(null) }
    val keepFailed = stringResource(R.string.folder_keep_failed)
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocumentTree()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        pickError = null
        // A folder on device storage is used by path when Carryover may read
        // every file (faster); any other folder (an emulator's own, or without
        // that access) through the Storage Access Framework.
        val path = if (Storage.hasAccess(ctx)) Storage.treeUriToPath(uri) else null
        if (path != null) {
            onChange(path)
            return@rememberLauncherForActivityResult
        }
        val flags = Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        val kept = runCatching { ctx.contentResolver.takePersistableUriPermission(uri, flags) }.isSuccess
        val authority = uri.authority
        if (!kept || authority == null) {
            pickError = keepFailed
            return@rememberLauncherForActivityResult
        }
        onChange(SafPath.of(authority, DocumentsContract.getTreeDocumentId(uri)))
    }
    // A granted folder has no path to edit: it shows where it is; Browse picks another.
    val saf = SafPath.isSaf(value)
    Column(Modifier.fillMaxWidth()) {
        OutlinedTextField(
            value = if (saf) SafPath.describe(value) { providerLabel(ctx, it) } else value,
            onValueChange = if (saf) {
                {}
            } else {
                {
                    pickError = null
                    onChange(it)
                }
            },
            readOnly = saf,
            label = { Text(label) },
            singleLine = true,
            isError = isError || pickError != null,
            supportingText = (pickError ?: supporting)?.let { { Text(it) } },
            trailingIcon = {
                TextButton(onClick = { picker.launch(null) }) { Text(stringResource(R.string.action_browse)) }
            },
            modifier = Modifier.fillMaxWidth().dpadExit(),
        )
        if (note != null || saf || actions != null) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    note.orEmpty(),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.weight(1f).padding(start = 16.dp),
                )
                if (saf) {
                    IconButton(onClick = { onChange("") }) {
                        Icon(Icons.Outlined.Keyboard, contentDescription = stringResource(R.string.action_type_path))
                    }
                }
                actions?.invoke(this)
            }
        }
    }
}

/**
 * A save folder as the game's page shows it: a folder picked through the
 * system by where it is ("Internal storage › Games/PSP/SAVEDATA"), a path as
 * the core resolved it ([resolved], tokens filled in), else as stored.
 */
@Composable
fun folderLocation(value: String, resolved: String?): String {
    val ctx = LocalContext.current
    return if (SafPath.isSaf(value)) SafPath.describe(value) { providerLabel(ctx, it) } else resolved?.ifBlank { null } ?: value
}

/** The name of the app that publishes a documents provider (an emulator's data folder). */
private fun providerLabel(ctx: Context, authority: String): String? = runCatching {
    val pm = ctx.packageManager
    pm.resolveContentProvider(authority, 0)?.loadLabel(pm)?.toString()
}.getOrNull()
