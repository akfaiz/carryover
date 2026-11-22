package com.akfaiz.carryover.ui

import android.content.Context
import android.content.Intent
import android.provider.DocumentsContract
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SafPath
import com.akfaiz.carryover.core.Storage

/**
 * A game file with a Browse button: the system file picker gives a content://
 * URI, which this app keeps permission to read, so it can let the emulator
 * read it when the game starts. A typed path works too.
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
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        singleLine = true,
        supportingText = { Text(hint ?: stringResource(R.string.game_file_hint)) },
        trailingIcon = { TextButton(onClick = { picker.launch(arrayOf("*/*")) }) { Text(stringResource(R.string.action_browse)) } },
        modifier = Modifier.fillMaxWidth().dpadExit(),
    )
}

/**
 * A folder path with a Browse button. The system picker returns a document
 * URI; phase 1 turns it into a path, so only folders on device storage work.
 */
@Composable
fun FolderField(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    supporting: String? = null,
    isError: Boolean = false,
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
    Column(Modifier.fillMaxWidth()) {
        if (SafPath.isSaf(value)) {
            // A granted folder has no path to edit: show where it is; Browse picks another.
            OutlinedTextField(
                value = SafPath.describe(value) { providerLabel(ctx, it) },
                onValueChange = {},
                readOnly = true,
                label = { Text(label) },
                singleLine = true,
                isError = isError || pickError != null,
                supportingText = { Text(pickError ?: supporting ?: stringResource(R.string.folder_granted)) },
                trailingIcon = {
                    TextButton(onClick = { picker.launch(null) }) { Text(stringResource(R.string.action_browse)) }
                },
                modifier = Modifier.fillMaxWidth().dpadExit(),
            )
            TextButton(onClick = { onChange("") }) { Text(stringResource(R.string.action_type_path)) }
            return@Column
        }
        OutlinedTextField(
            value = value,
            onValueChange = {
                pickError = null
                onChange(it)
            },
            label = { Text(label) },
            singleLine = true,
            isError = isError || pickError != null,
            supportingText = (pickError ?: supporting)?.let { { Text(it) } },
            trailingIcon = {
                TextButton(onClick = { picker.launch(null) }) { Text(stringResource(R.string.action_browse)) }
            },
            modifier = Modifier.fillMaxWidth().dpadExit(),
        )
    }
}

/** The name of the app that publishes a documents provider (an emulator's data folder). */
private fun providerLabel(ctx: Context, authority: String): String? = runCatching {
    val pm = ctx.packageManager
    pm.resolveContentProvider(authority, 0)?.loadLabel(pm)?.toString()
}.getOrNull()
