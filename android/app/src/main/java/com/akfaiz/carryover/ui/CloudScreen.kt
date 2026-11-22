package com.akfaiz.carryover.ui

import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
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
import com.akfaiz.carryover.R
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.core.OptionExample
import com.akfaiz.carryover.core.Provider
import com.akfaiz.carryover.core.ProviderOption
import com.akfaiz.carryover.core.Question
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BackBar(title: String, nav: Nav) {
    TopAppBar(
        title = { Text(title) },
        navigationIcon = {
            IconButton(onClick = nav::back) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back))
            }
        },
    )
}

/** Picks the remote and folder Carryover syncs with. */
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
    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { BackBar(stringResource(R.string.section_cloud), nav) }) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(stringResource(R.string.cloud_intro), style = MaterialTheme.typography.bodyMedium)
            Text(stringResource(R.string.cloud_accounts), style = MaterialTheme.typography.titleSmall)
            if (state.remotes.isEmpty()) {
                Text(stringResource(R.string.cloud_none), style = MaterialTheme.typography.bodySmall)
            }
            for (r in state.remotes) {
                Row(
                    Modifier.fillMaxWidth().selectable(
                        selected = remote == r.name,
                        enabled = !r.incomplete,
                        onClick = { remote = r.name },
                    ),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    RadioButton(selected = remote == r.name, onClick = { remote = r.name }, enabled = !r.incomplete)
                    Column(Modifier.weight(1f)) {
                        Text(r.name)
                        Text(r.type, style = MaterialTheme.typography.bodySmall)
                        if (r.incomplete) {
                            Text(
                                stringResource(R.string.cloud_signin_unfinished),
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.error,
                            )
                        }
                    }
                    IconButton(onClick = { vm.deleteRemote(r.name) }) {
                        Icon(Icons.Filled.Delete, contentDescription = stringResource(R.string.action_delete_remote, r.name))
                    }
                }
            }
            OutlinedButton(onClick = { nav.go(Screen.AddRemote) }) { Text(stringResource(R.string.action_add_account)) }
            // Accounts of an rclone.conf from a PC: no sign-in on the phone.
            OutlinedButton(onClick = { pickConfig.launch(arrayOf("*/*")) }) { Text(stringResource(R.string.action_import_config)) }
            Text(
                stringResource(R.string.cloud_import_hint),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            OutlinedTextField(
                root,
                { root = it },
                label = { Text(stringResource(R.string.cloud_folder)) },
                supportingText = { Text(stringResource(R.string.cloud_folder_hint)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Button(
                onClick = {
                    vm.useCloud(remote, root.trim())
                    nav.back()
                },
                enabled = remote.isNotEmpty() && root.isNotBlank(),
            ) { Text(stringResource(R.string.action_use_cloud)) }
        }
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
            error = it.message
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
                    error = q.error.ifEmpty { null }
                }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (cancelled) {
                    // Back to the options: the core dropped the account it had begun.
                    question = null
                    vm.refresh()
                } else {
                    error = e.message ?: e.toString()
                }
            } finally {
                working = false
                cancelled = false
                vm.clearAuthUrl()
            }
        }
    }

    Scaffold(contentWindowInsets = LocalPageInsets.current, topBar = { BackBar(stringResource(R.string.action_add_account), nav) }) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            val p = provider
            val q = question
            when {
                working -> {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        CircularProgressIndicator()
                        Text(stringResource(if (authUrl != null) R.string.add_account_signin else R.string.add_account_connecting))
                    }
                    authUrl?.let { url ->
                        Text(
                            stringResource(R.string.add_account_browser_hint),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        Text(url, style = MaterialTheme.typography.bodySmall)
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            OutlinedButton(onClick = { vm.openBrowser(url) }) { Text(stringResource(R.string.action_open_browser)) }
                            // Waiting for the browser is the only step that can hang: this ends it.
                            TextButton(onClick = {
                                cancelled = true
                                vm.cancelSignIn()
                            }) { Text(stringResource(R.string.action_cancel)) }
                        }
                    }
                }
                p == null -> {
                    Text(stringResource(R.string.add_account_where), style = MaterialTheme.typography.titleSmall)
                    for (pr in providers) {
                        Card(
                            Modifier.fillMaxWidth().clickable {
                                provider = pr
                                name = defaultRemoteName(pr.type, vm.state.value.remotes.map { it.name }.toSet())
                                params.clear()
                            },
                        ) {
                            Column(Modifier.padding(16.dp)) {
                                Text(pr.description.ifEmpty { pr.type }, style = MaterialTheme.typography.titleMedium)
                                Text(pr.type, style = MaterialTheme.typography.bodySmall)
                            }
                        }
                    }
                }
                q != null -> {
                    Text(q.help.ifEmpty { q.name }, style = MaterialTheme.typography.bodyMedium)
                    if (q.examples.isNotEmpty()) {
                        Choices(q.examples, answer) { answer = it }
                    }
                    if (!(q.exclusive && q.examples.isNotEmpty())) {
                        OptionField(q.name, answer, q.isPassword) { answer = it }
                    }
                    Button(onClick = { step { vm.continueRemote(name.trim(), q.state, answer) } }) { Text(stringResource(R.string.action_continue)) }
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
                        TextButton(onClick = { showAll = true }) { Text(stringResource(R.string.action_more_options)) }
                    }
                    if (p.oauth) {
                        Text(
                            stringResource(R.string.add_account_oauth, p.description.ifEmpty { p.type }),
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(
                            onClick = { step { vm.createRemote(name.trim(), p.type, params.toMap()) } },
                            enabled = name.isNotBlank(),
                        ) { Text(stringResource(R.string.action_connect)) }
                        OutlinedButton(onClick = { provider = null }) { Text(stringResource(R.string.action_other_provider)) }
                    }
                }
            }
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        }
    }
}

@Composable
private fun OptionInput(o: ProviderOption, value: String, onChange: (String) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        if (o.help.isNotEmpty()) {
            Text(o.help.lineSequence().first(), style = MaterialTheme.typography.bodySmall)
        }
        if (o.exclusive && o.examples.isNotEmpty()) {
            Choices(o.examples, value.ifEmpty { o.default }, onChange)
        } else {
            OptionField(o.name + if (o.required) " *" else "", value, o.isPassword, onChange)
        }
    }
}

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
            Row(
                Modifier.fillMaxWidth().selectable(selected = selected == e.value, onClick = { onSelect(e.value) }),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RadioButton(selected = selected == e.value, onClick = { onSelect(e.value) })
                Column {
                    Text(e.value.ifEmpty { stringResource(R.string.option_empty) })
                    if (e.help.isNotEmpty()) Text(e.help, style = MaterialTheme.typography.bodySmall)
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
