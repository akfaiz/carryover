package com.akfaiz.carryover.ui.cloud

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.OptionExample
import com.akfaiz.carryover.core.Provider
import com.akfaiz.carryover.core.ProviderOption
import com.akfaiz.carryover.core.Question
import com.akfaiz.carryover.data.core
import com.akfaiz.carryover.data.error
import com.akfaiz.carryover.ui.components.rememberTexts
import com.akfaiz.carryover.ui.components.SettingRow
import com.akfaiz.carryover.ui.components.readLimited
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Callout
import com.akfaiz.carryover.ui.kit.ChoiceRow
import com.akfaiz.carryover.ui.kit.EmptyState
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.ListRows
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.Spinner
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.kit.color
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.defaultRemoteName
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/** Picks the remote and folder Carryover syncs with. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun CloudScreen(nav: Nav, vm: CloudViewModel = viewModel()) {
    val state by vm.state.collectAsStateWithLifecycle()
    var remote by remember(state.settings.remote) { mutableStateOf(state.settings.remote) }
    // The accounts load after the page opens (and one may just have been added):
    // with none chosen yet, take the first.
    // An account whose sign-in never finished can't be used: it is never the one taken or kept.
    LaunchedEffect(state.remotes) {
        if (state.remotes.none { it.name == remote && !it.incomplete }) {
            remote = state.remotes.firstOrNull { !it.incomplete }?.name.orEmpty()
        }
    }
    var root by remember(state.settings.remoteRoot) { mutableStateOf(state.settings.remoteRoot.ifEmpty { "Carryover" }) }
    // Connecting runs a library sync: the page waits for it, and stays open if it fails.
    var connecting by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf<String?>(null) }
    val active = if (state.cloudReady) state.settings.remote else ""
    LaunchedEffect(Unit) { vm.refresh() }
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val scope = rememberCoroutineScope()
    // The user picks the rclone.conf; the app only gets its contents (nothing is copied to a file of ours).
    val pickConfig = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.OpenDocument(),
    ) { uri ->
        if (uri != null) {
            scope.launch {
                try {
                    vm.importConfig(readConfigText(ctx, uri))
                } catch (e: Exception) {
                    if (e is CancellationException) throw e
                    vm.importUnreadable(e)
                }
            }
        }
    }
    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { PageTopBar(stringResource(R.string.section_cloud), nav::back) }) { padding ->
        PageColumn(padding) {
            Hint(stringResource(R.string.cloud_intro))
            SectionCard(stringResource(R.string.cloud_accounts)) {
                if (state.remotes.isEmpty()) {
                    EmptyState(Icons.Outlined.Cloud, stringResource(R.string.cloud_none), stringResource(R.string.cloud_none_text))
                }
                // Drawn when empty too, for the last account to be seen going.
                ListRows(state.remotes, key = { it.name }) { r ->
                    ChoiceRow(remote == r.name, { remote = r.name }, enabled = !r.incomplete) {
                        Column(Modifier.weight(1f)) {
                            Text(r.name, style = MaterialTheme.typography.bodyLarge)
                            Text(
                                if (r.name == active) "${r.type} · ${stringResource(R.string.cloud_in_use)}" else r.type,
                                style = MaterialTheme.typography.bodySmall,
                                color = if (r.name == active) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            if (r.incomplete) {
                                Text(
                                    stringResource(R.string.cloud_signin_unfinished),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Tone.Err.color(),
                                )
                            }
                        }
                        IconButton(onClick = { deleting = r.name }) {
                            Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.action_delete_remote, r.name))
                        }
                    }
                }
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    AppButton(onClick = { nav.go(Screen.AddRemote) }, icon = Icons.Outlined.Add) { Text(stringResource(R.string.action_add_account)) }
                    // Accounts of an rclone.conf from a PC: no sign-in on the phone.
                    AppButton(onClick = { pickConfig.launch(arrayOf("*/*")) }, kind = BtnKind.Secondary) {
                        Text(stringResource(R.string.action_import_config))
                    }
                }
                Hint(stringResource(R.string.cloud_import_hint))
            }
            SectionCard(null) {
                OutlinedTextField(
                    root,
                    { root = it },
                    label = { Text(stringResource(R.string.cloud_folder)) },
                    supportingText = { Text(stringResource(R.string.cloud_folder_hint)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    AppButton(
                        onClick = {
                            connecting = true
                            vm.useCloud(remote, root.trim()) { ok ->
                                connecting = false
                                if (ok) nav.back()
                            }
                        },
                        enabled = !connecting && remote.isNotEmpty() && root.isNotBlank(),
                    ) { Text(stringResource(R.string.action_use_cloud)) }
                    if (connecting) Spinner()
                }
            }
        }
    }
    deleting?.let { name ->
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text(stringResource(R.string.dialog_delete_remote_title, name)) },
            text = {
                Text(stringResource(if (name == active) R.string.dialog_delete_remote_active else R.string.dialog_delete_remote_text))
            },
            confirmButton = {
                AppButton(onClick = {
                    deleting = null
                    vm.deleteRemote(name)
                }, kind = BtnKind.Danger) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { AppButton(onClick = { deleting = null }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/**
 * Creates an rclone remote: pick a provider, fill in its main options, then
 * answer rclone's questions. OAuth providers open the browser and wait for
 * the sign-in to finish (the desktop app's wizard works the same way).
 */
@Composable
fun AddRemoteScreen(nav: Nav, vm: CloudViewModel = viewModel()) {
    val scope = rememberCoroutineScope()
    val texts = rememberTexts()
    var providers by remember { mutableStateOf<List<Provider>>(emptyList()) }
    var provider by remember { mutableStateOf<Provider?>(null) }
    var name by remember { mutableStateOf("") }
    val params = remember { mutableStateMapOf<String, String>() }
    var showAll by remember { mutableStateOf(false) }
    var question by remember { mutableStateOf<Question?>(null) }
    var answer by remember { mutableStateOf("") }
    var working by remember { mutableStateOf(false) }
    // The user gave up waiting for the browser: the step then ends in an error that isn't one to show.
    var cancelled by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val authUrl by vm.authUrl.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        providers = runCatching { vm.providers() }.getOrElse {
            error = texts.error(it)
            emptyList()
        }
    }

    fun step(work: suspend () -> Question) {
        working = true
        cancelled = false
        error = null
        scope.launch {
            try {
                val q = work()
                if (q.done) {
                    vm.refresh()
                    nav.back()
                } else {
                    question = q
                    answer = q.default
                    error = texts.core(q.error, q.errorMsg).ifEmpty { null }
                }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (cancelled) {
                    // Back to the options: the core dropped the account it had begun.
                    question = null
                    vm.refresh()
                } else {
                    error = texts.error(e)
                }
            } finally {
                working = false
                cancelled = false
                vm.clearAuthUrl()
            }
        }
    }

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { PageTopBar(stringResource(R.string.action_add_account), nav::back) }) { padding ->
        PageColumn(padding) {
            val p = provider
            val q = question
            when {
                working -> {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Spinner()
                        Text(stringResource(if (authUrl != null) R.string.add_account_signin else R.string.add_account_connecting))
                    }
                    authUrl?.let { url ->
                        Hint(stringResource(R.string.add_account_browser_hint))
                        Hint(url)
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            AppButton(onClick = { vm.openBrowser(url) }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_open_browser)) }
                            // Waiting for the browser is the only step that can hang: this ends it.
                            AppButton(onClick = {
                                cancelled = true
                                vm.cancelSignIn()
                            }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) }
                        }
                    }
                }
                p == null -> {
                    // The title and the rows share the 16dp inset the rows carry themselves.
                    SectionCard(null, contentPadding = PaddingValues(vertical = 8.dp), gap = 0.dp) {
                        Text(
                            stringResource(R.string.add_account_where),
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                        )
                        for (pr in providers) {
                            SettingRow(
                                icon = Icons.Outlined.Cloud,
                                title = pr.description.ifEmpty { pr.type },
                                onClick = {
                                    provider = pr
                                    name = defaultRemoteName(pr.type, vm.state.value.remotes.map { it.name }.toSet())
                                    params.clear()
                                },
                            )
                        }
                    }
                }
                q != null -> {
                    Text(q.help.ifEmpty { q.name }, style = MaterialTheme.typography.bodyMedium)
                    if (q.examples.isNotEmpty()) {
                        Choices(q.examples, answer) { answer = it }
                    }
                    if (!(q.exclusive && q.examples.isNotEmpty())) {
                        OptionField(optionLabel(q.name), answer, q.isPassword) { answer = it }
                    }
                    AppButton(onClick = { step { vm.continueRemote(name.trim(), q.state, answer) } }) { Text(stringResource(R.string.action_continue)) }
                }
                else -> {
                    Text(p.description.ifEmpty { p.type }, style = MaterialTheme.typography.titleMedium)
                    OutlinedTextField(
                        name,
                        { name = it },
                        label = { Text(stringResource(R.string.field_name)) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    val visible = p.options.filter { o ->
                        showAll || o.required || o.name == "provider" || (o.exclusive && o.examples.isNotEmpty())
                    }
                    for (o in visible) {
                        OptionInput(o, params[o.name] ?: "") { params[o.name] = it }
                    }
                    if (visible.size < p.options.size) {
                        AppButton(onClick = { showAll = true }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_more_options)) }
                    }
                    if (p.oauth) {
                        Hint(stringResource(R.string.add_account_oauth, p.description.ifEmpty { p.type }))
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        AppButton(
                            onClick = { step { vm.createRemote(name.trim(), p.type, params.toMap()) } },
                            enabled = name.isNotBlank(),
                        ) { Text(stringResource(R.string.action_connect)) }
                        AppButton(onClick = { provider = null }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_other_provider)) }
                    }
                }
            }
            error?.let { Callout(Tone.Err, Icons.Outlined.ErrorOutline, it) }
        }
    }
}

@Composable
private fun OptionInput(o: ProviderOption, value: String, onChange: (String) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        if (o.help.isNotEmpty()) {
            Hint(o.help.lineSequence().first())
        }
        if (o.exclusive && o.examples.isNotEmpty()) {
            Choices(o.examples, value.ifEmpty { o.default }, onChange)
        } else {
            OptionField(optionLabel(o.name) + if (o.required) " *" else "", value, o.isPassword, onChange)
        }
    }
}

/** An rclone option's name as a label: "client_id" → "Client id" (rclone has no display names). */
internal fun optionLabel(name: String): String = name.replace('_', ' ').replaceFirstChar { it.uppercase() }

@Composable
private fun OptionField(label: String, value: String, password: Boolean, onChange: (String) -> Unit) {
    OutlinedTextField(
        value,
        onChange,
        label = { Text(label) },
        singleLine = true,
        visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        keyboardOptions = if (password) KeyboardOptions(keyboardType = KeyboardType.Password) else KeyboardOptions.Default,
        modifier = Modifier.fillMaxWidth(),
    )
}

@Composable
private fun Choices(examples: List<OptionExample>, selected: String, onSelect: (String) -> Unit) {
    Column {
        for (e in examples) {
            ChoiceRow(selected == e.value, { onSelect(e.value) }) {
                Column {
                    Text(e.value.ifEmpty { stringResource(R.string.option_empty) })
                    if (e.help.isNotEmpty()) Hint(e.help)
                }
            }
        }
    }
}

/** The most an rclone.conf can be: they are a few kilobytes, so anything bigger is not one. */
private const val MAX_CONFIG_BYTES = 1 shl 20

/** The text of the rclone.conf at [uri]. */
private suspend fun readConfigText(ctx: android.content.Context, uri: android.net.Uri): String = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
    val bytes = ctx.contentResolver.openInputStream(uri)?.use { readLimited(it, MAX_CONFIG_BYTES) }
        ?: throw java.io.IOException(ctx.getString(R.string.err_import_too_big))
    String(bytes, Charsets.UTF_8)
}
