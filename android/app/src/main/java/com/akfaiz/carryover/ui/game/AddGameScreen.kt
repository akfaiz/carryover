package com.akfaiz.carryover.ui.game

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.ui.components.AppIcon
import com.akfaiz.carryover.ui.components.AppPickerDialog
import com.akfaiz.carryover.ui.components.FolderField
import com.akfaiz.carryover.ui.components.GameFileField
import com.akfaiz.carryover.ui.components.appLabel
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.ChoiceRow
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.dpadExit
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState
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

    // What is still missing, first in page order: said under Add instead of only greying it out.
    val missing = when {
        emulator.isEmpty() -> R.string.add_game_need_emulator
        android && app.isBlank() -> R.string.add_game_need_app
        name.isBlank() -> R.string.add_game_need_name
        android && savePath.isBlank() -> R.string.add_game_need_folder
        else -> null
    }
    val ready = missing == null
    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(stringResource(R.string.add_game_title), nav::back) },
    ) { padding ->
        PageColumn(padding) {
            SectionCard(stringResource(R.string.add_game_emulator)) {
                if (emulators.isEmpty()) {
                    Text(
                        stringResource(R.string.add_game_no_emulators),
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    AppButton(onClick = { nav.go(Screen.Emulators) }, kind = BtnKind.Secondary) {
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
                    AppButton(onClick = { picking = true }, kind = BtnKind.Secondary) {
                        Text(stringResource(if (app.isBlank()) R.string.action_choose_game_app else R.string.action_change_app))
                    }
                }
            } else if (emulator.isNotEmpty()) {
                SectionCard(stringResource(R.string.game_file)) {
                    GameFileField(rom, { rom = it }, stringResource(R.string.game_file), hint = stringResource(R.string.add_game_rom_hint))
                }
            }

            SectionCard(null) {
                OutlinedTextField(
                    name,
                    {
                        name = it
                        nameTyped = it.isNotBlank()
                    },
                    label = { Text(stringResource(R.string.field_name)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth().dpadExit(),
                )
            }

            SectionCard(stringResource(R.string.field_save_folder)) {
                if (android) {
                    Text(
                        stringResource(R.string.add_game_android_note),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                FolderField(
                    savePath,
                    { savePath = it },
                    stringResource(R.string.field_save_folder),
                    supporting = stringResource(if (android) R.string.add_game_save_required else R.string.add_game_save_optional),
                )
            }

            AppButton(
                large = true,
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
                modifier = Modifier.fillMaxWidth(),
            ) { Text(stringResource(R.string.action_add)) }
            missing?.let { Hint(stringResource(it), Modifier.align(Alignment.CenterHorizontally)) }
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

/** The file name of a game file: a content:// URI's display name, or a path's last part. */
suspend fun fileName(ctx: Context, rom: String): String = withContext(Dispatchers.IO) {
    if (!rom.startsWith("content://")) return@withContext rom.substringAfterLast('/')
    runCatching {
        ctx.contentResolver.query(Uri.parse(rom), arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) c.getString(0) else null
        }
    }.getOrNull() ?: Uri.decode(rom).substringAfterLast('/').substringAfterLast(':')
}
