package com.akfaiz.carryover.ui

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorView
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** "Android game" in place of an emulator id. */
private const val ANDROID_GAME = ":android"

/**
 * Adds a game: the emulator it is played with (one set up on this device),
 * its game file, whose name fills in the game's name, and optionally its save
 * folder. An Android game takes its own app and a save folder instead.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AddGameScreen(state: UiState, vm: AppViewModel, nav: Nav) {
    val ctx = LocalContext.current
    // Only emulators this device can start: those with an app chosen here.
    val emulators = state.emulators.filter { it.device.exe.isNotBlank() }
    var emulator by rememberSaveable { mutableStateOf(emulators.singleOrNull()?.id.orEmpty()) }
    var rom by rememberSaveable { mutableStateOf("") }
    var name by rememberSaveable { mutableStateOf("") }
    var nameTyped by rememberSaveable { mutableStateOf(false) }
    var savePath by rememberSaveable { mutableStateOf("") }
    var app by rememberSaveable { mutableStateOf("") }
    var picking by rememberSaveable { mutableStateOf(false) }
    val android = emulator == ANDROID_GAME

    // The game file's name suggests the game's (a Switch title ID gives the real title).
    LaunchedEffect(rom, emulator) {
        if (rom.isBlank() || nameTyped || android || emulator.isEmpty()) return@LaunchedEffect
        val guess = vm.guessRomName(emulator, fileName(ctx, rom))
        if (guess.isNotBlank()) name = guess
    }

    val ready = name.isNotBlank() && when {
        android -> app.isNotBlank() && savePath.isNotBlank()
        else -> emulator.isNotEmpty()
    }
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.add_game_title)) },
                navigationIcon = {
                    IconButton(onClick = nav::back) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back))
                    }
                },
                windowInsets = pageBarInsets(),
            )
        },
    ) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(
                Modifier.widthIn(max = 760.dp).fillMaxWidth().padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                SectionCard(stringResource(R.string.add_game_emulator)) {
                    if (emulators.isEmpty()) {
                        Text(
                            stringResource(R.string.add_game_no_emulators),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        OutlinedButton(onClick = { nav.go(Screen.Emulators) }) {
                            Text(stringResource(R.string.action_set_up_emulators))
                        }
                    }
                    for (e in emulators) {
                        EmulatorChoice(e, emulator == e.id) { emulator = e.id }
                    }
                    AndroidChoice(android) { emulator = ANDROID_GAME }
                }

                if (android) {
                    SectionCard(stringResource(R.string.game_launch)) {
                        Text(
                            if (app.isBlank()) stringResource(R.string.game_app_none) else stringResource(R.string.game_app, appLabel(ctx, app)),
                            style = MaterialTheme.typography.bodyMedium,
                        )
                        OutlinedButton(onClick = { picking = true }) {
                            Text(stringResource(if (app.isBlank()) R.string.action_choose_game_app else R.string.action_change_app))
                        }
                    }
                } else if (emulator.isNotEmpty()) {
                    SectionCard(stringResource(R.string.game_file)) {
                        GameFileField(rom, { rom = it }, stringResource(R.string.game_file), hint = stringResource(R.string.add_game_rom_hint))
                    }
                }

                SectionCard(stringResource(R.string.field_name)) {
                    OutlinedTextField(
                        name,
                        {
                            name = it
                            nameTyped = it.isNotBlank()
                        },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth().dpadExit(),
                    )
                }

                SectionCard(stringResource(R.string.field_save_folder)) {
                    FolderField(
                        savePath,
                        { savePath = it },
                        stringResource(R.string.field_save_folder),
                        supporting = if (android) null else stringResource(R.string.add_game_save_optional),
                    )
                }

                Button(
                    onClick = {
                        val open: (String) -> Unit = { id ->
                            nav.back()
                            nav.go(Screen.Game(id))
                        }
                        if (android) {
                            vm.addAndroidGame(name, app, savePath, open)
                        } else {
                            vm.addEmulatorGame(name, emulator, rom, savePath, open)
                        }
                    },
                    enabled = ready,
                    modifier = Modifier.fillMaxWidth().height(52.dp),
                ) { Text(stringResource(R.string.action_add)) }
            }
        }
    }
    if (picking) {
        AppPickerDialog(stringResource(R.string.action_choose_game_app), onDismiss = { picking = false }) { a ->
            picking = false
            app = a.pkg
            if (name.isBlank()) name = a.label
        }
    }
}

@Composable
private fun EmulatorChoice(e: EmulatorView, selected: Boolean, onClick: () -> Unit) {
    val ctx = LocalContext.current
    ChoiceRow(selected, onClick) {
        AppIcon(e.device.exe.substringBefore('/'), 36)
        Column(Modifier.weight(1f)) {
            Text(e.name, style = MaterialTheme.typography.bodyLarge)
            Text(
                appLabel(ctx, e.device.exe),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun AndroidChoice(selected: Boolean, onClick: () -> Unit) {
    ChoiceRow(selected, onClick) {
        Icon(Icons.Outlined.PhoneAndroid, null, Modifier.padding(horizontal = 6.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(stringResource(R.string.add_game_android), style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))
    }
}

/** A selectable row with a radio button. */
@Composable
fun ChoiceRow(selected: Boolean, onClick: () -> Unit, content: @Composable RowScope.() -> Unit) {
    val shape = RoundedCornerShape(12.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .focusRing(shape)
            .clip(shape)
            .selectable(selected = selected, onClick = onClick)
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        RadioButton(selected = selected, onClick = onClick)
        content()
    }
}

/** The file name of a game file: a content:// URI's display name, or a path's last part. */
suspend fun fileName(ctx: Context, rom: String): String = withContext(Dispatchers.IO) {
    if (!rom.startsWith("content://")) return@withContext rom.substringAfterLast('/')
    runCatching {
        ctx.contentResolver.query(Uri.parse(rom), arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) c.getString(0) else null
        }
    }.getOrNull() ?: Uri.decode(rom).substringAfterLast('/').substringAfterLast(':')
}
