package com.akfaiz.carryover.ui.emulators

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.os.Build
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorPresets
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.launch.FrontendConfig
import com.akfaiz.carryover.launch.LaunchActivity
import com.akfaiz.carryover.ui.components.AppIcon
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Callout
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageColumn
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState

/**
 * How to start games from a frontend (ES-DE, Daijisho, any app) through
 * Carryover, with the settings to copy made for one emulator set up on this
 * device (FrontendConfig). The same as docs/android-frontends.md.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FrontendsScreen(emulatorId: String, state: UiState, vm: AppViewModel, nav: Nav) {
    val ctx = LocalContext.current
    val emulators = state.emulators.filter { it.device.exe.isNotBlank() }
    var chosen by rememberSaveable { mutableStateOf(emulatorId.ifEmpty { emulators.firstOrNull()?.id.orEmpty() }) }
    var tab by rememberSaveable { mutableIntStateOf(0) }
    val emu = emulators.firstOrNull { it.id == chosen } ?: emulators.firstOrNull()
    val carryover = "${ctx.packageName}/${LaunchActivity::class.java.name}"
    val config = remember(emu) { emu?.let { runCatching { frontendEmulator(ctx, it) } } }

    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = { PageTopBar(stringResource(R.string.frontends_title), nav::back) },
    ) { padding ->
        PageColumn(padding) {
            SectionCard(null) {
                Text(stringResource(R.string.frontends_intro), style = MaterialTheme.typography.bodyMedium)
                Hint(stringResource(R.string.frontends_needs))
            }

            SectionCard(stringResource(R.string.frontends_for)) {
                if (emulators.isEmpty()) {
                    Hint(stringResource(R.string.add_game_no_emulators))
                    AppButton(onClick = { nav.go(Screen.Emulators) }, kind = BtnKind.Secondary) {
                        Text(stringResource(R.string.action_set_up_emulators))
                    }
                }
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (e in emulators) {
                        FilterChip(
                            selected = e.id == emu?.id,
                            onClick = { chosen = e.id },
                            label = { Text(e.name, maxLines = 1) },
                            leadingIcon = { AppIcon(e.device.exe.substringBefore('/'), 20) },
                        )
                    }
                }
            }

            if (emu != null) {
                val tabs = listOf(R.string.frontend_esde, R.string.frontend_daijisho, R.string.frontend_other)
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    tabs.forEachIndexed { i, label ->
                        SegmentedButton(
                            selected = tab == i,
                            onClick = { tab = i },
                            shape = SegmentedButtonDefaults.itemShape(i, tabs.size),
                            colors = SegmentedButtonDefaults.colors(
                                activeContainerColor = MaterialTheme.colorScheme.primaryContainer,
                                activeContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                            ),
                            icon = {},
                        ) { Text(stringResource(label), maxLines = 1) }
                    }
                }
                val e = config?.getOrNull()
                if (e == null) {
                    // The emulator's start template can't be read: fix it on its page.
                    Callout(
                        Tone.Err,
                        Icons.Outlined.ErrorOutline,
                        stringResource(R.string.frontends_bad_template, emu.name, config?.exceptionOrNull()?.message.orEmpty()),
                    )
                } else {
                    val copy = { label: String, text: String -> copyText(ctx, label, text, vm) }
                    when (tab) {
                        0 -> SectionCard(stringResource(R.string.frontend_esde)) {
                            Step(stringResource(R.string.esde_step_rules))
                            CodeBlock(FrontendConfig.esdeFindRules(carryover)) { copy("es_find_rules.xml", it) }
                            Step(stringResource(R.string.esde_step_command))
                            CodeBlock(FrontendConfig.esdeCommand(e)) { copy("es_systems.xml", it) }
                            Step(stringResource(R.string.esde_step_choose))
                        }
                        1 -> SectionCard(stringResource(R.string.frontend_daijisho)) {
                            Step(stringResource(R.string.daijisho_step_player))
                            Field(stringResource(R.string.daijisho_field_name))
                            CodeBlock(FrontendConfig.daijishoName(e)) { copy("Player name", it) }
                            Field(stringResource(R.string.daijisho_field_args))
                            CodeBlock(FrontendConfig.daijishoArgs(e, carryover)) { copy("Player am start arguments", it) }
                            Field(stringResource(R.string.daijisho_field_regex))
                            CodeBlock(FrontendConfig.daijishoRegex(fileTypes(emu))) { copy("Player accepted filename regex", it) }
                            Hint(stringResource(R.string.daijisho_regex_hint))
                            Step(stringResource(R.string.daijisho_step_choose))
                        }
                        else -> SectionCard(stringResource(R.string.frontend_other)) {
                            Step(stringResource(R.string.other_step))
                            Step(stringResource(R.string.other_target))
                            CodeBlock("carryover.target=${e.target}") { copy("carryover.target", e.target) }
                            Step(stringResource(R.string.other_adb))
                            CodeBlock(FrontendConfig.adbCommand(e, carryover)) { copy("adb", it) }
                        }
                    }
                    Hint(stringResource(R.string.frontends_untested))
                }
            }

            SectionCard(stringResource(R.string.shortcuts_title)) {
                Text(stringResource(R.string.shortcuts_text), style = MaterialTheme.typography.bodyMedium)
            }
        }
    }
}

/** The frontend launch of an emulator set up here: its template, else its preset's, else "open this file". */
private fun frontendEmulator(ctx: Context, e: EmulatorView): FrontendConfig.Emulator {
    val exe = e.device.exe
    val pkg = exe.substringBefore('/')
    val template = e.device.args.ifBlank { EmulatorPresets.forPackage(pkg)?.template.orEmpty() }
    val pm = ctx.packageManager
    val launcher = (pm.getLaunchIntentForPackage(pkg) ?: pm.getLeanbackLaunchIntentForPackage(pkg))?.component?.className
    return FrontendConfig.emulator(e.name, exe, template, launcher) // throws LaunchException for a bad template
}

/** The game files an emulator opens: its app's preset, else its kind's (the desktop's preset). */
private fun fileTypes(e: EmulatorView): List<String> =
    EmulatorPresets.forPackage(e.device.exe.substringBefore('/'))?.extensions?.takeIf { it.isNotEmpty() }
        ?: EmulatorPresets.all.firstOrNull { it.manifestPreset.isNotEmpty() && it.manifestPreset == e.preset }?.extensions
        ?: emptyList()

private fun copyText(ctx: Context, label: String, text: String, vm: AppViewModel) {
    ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText(label, text))
    // Android 13+ confirms a copy itself.
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) vm.copied()
}

@Composable
private fun Step(text: String) {
    Text(text, style = MaterialTheme.typography.bodyMedium)
}

/** The name of the frontend's field a value goes in, as the frontend shows it. */
@Composable
private fun Field(text: String) {
    Text(text, style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
}

/** A value to copy: monospace, scrolling sideways, with a Copy button. */
@Composable
private fun CodeBlock(text: String, onCopy: (String) -> Unit) {
    Surface(
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.surfaceContainerHighest,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column {
            Text(
                text,
                fontFamily = FontFamily.Monospace,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.horizontalScroll(rememberScrollState()).padding(start = 12.dp, end = 12.dp, top = 12.dp),
            )
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                AppButton(onClick = { onCopy(text) }, kind = BtnKind.Ghost, icon = Icons.Outlined.ContentCopy) {
                    Text(stringResource(R.string.action_copy))
                }
            }
        }
    }
}
