package com.akfaiz.carryover.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SportsEsports
import androidx.compose.material.icons.filled.VideoLibrary
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.SportsEsports
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationRail
import androidx.compose.material3.NavigationRailItem
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.ui.art.FindArtScreen
import com.akfaiz.carryover.ui.backups.BackupsScreen
import com.akfaiz.carryover.ui.cloud.AddRemoteScreen
import com.akfaiz.carryover.ui.cloud.CloudScreen
import com.akfaiz.carryover.ui.components.PromptDialog
import com.akfaiz.carryover.ui.components.describe
import com.akfaiz.carryover.ui.emulators.EmulatorScreen
import com.akfaiz.carryover.ui.emulators.EmulatorsScreen
import com.akfaiz.carryover.ui.emulators.FrontendsScreen
import com.akfaiz.carryover.ui.game.AddGameScreen
import com.akfaiz.carryover.ui.game.EditGameScreen
import com.akfaiz.carryover.ui.game.GameScreen
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.library.LibraryScreen
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.settings.SettingsScreen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.UiState
import com.akfaiz.carryover.ui.trash.TrashScreen

private data class Tab(val screen: Screen, val label: Int, val icon: ImageVector, val selectedIcon: ImageVector)

private val tabs = listOf(
    Tab(Screen.Library, R.string.nav_library, Icons.Outlined.VideoLibrary, Icons.Filled.VideoLibrary),
    Tab(Screen.Emulators, R.string.nav_emulators, Icons.Outlined.SportsEsports, Icons.Filled.SportsEsports),
    Tab(Screen.Settings, R.string.nav_settings, Icons.Outlined.Settings, Icons.Filled.Settings),
)

@Composable
fun CarryoverApp(vm: AppViewModel) {
    val state by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    val nav = rememberSaveable(saver = Nav.Saver) { Nav() }

    LaunchedEffect(Unit) {
        vm.messages.collect { snackbar.showSnackbar(it) }
    }
    state.initError?.let { err ->
        Surface(Modifier.fillMaxSize()) {
            Column(Modifier.safeDrawingPadding().padding(24.dp)) {
                Text(stringResource(R.string.init_failed_title), style = MaterialTheme.typography.headlineSmall)
                Text(err, Modifier.padding(top = 12.dp))
            }
        }
        return
    }
    BackHandler(enabled = nav.canGoBack) { nav.back() }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        BoxWithConstraints(Modifier.fillMaxSize()) {
            // Landscape handhelds and tablets: a rail on the side keeps the height for the games.
            val rail = maxWidth >= 600.dp && maxWidth > maxHeight
            val onTab = nav.stack.isEmpty()
            val insets = when {
                !onTab -> WindowInsets.safeDrawing
                rail -> WindowInsets.safeDrawing.only(WindowInsetsSides.Top + WindowInsetsSides.End + WindowInsetsSides.Bottom)
                else -> WindowInsets.safeDrawing.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal)
            }
            Row(Modifier.fillMaxSize()) {
                if (rail && onTab) {
                    NavigationRail(containerColor = MaterialTheme.colorScheme.surfaceContainer) {
                        Spacer(Modifier.weight(1f))
                        for (t in tabs) {
                            NavigationRailItem(
                                selected = nav.tab == t.screen,
                                onClick = { nav.go(t.screen) },
                                icon = { Icon(if (nav.tab == t.screen) t.selectedIcon else t.icon, null) },
                                label = { Text(stringResource(t.label)) },
                                modifier = Modifier.padding(vertical = 4.dp),
                            )
                        }
                        Spacer(Modifier.weight(1f))
                    }
                }
                Column(Modifier.weight(1f)) {
                    Box(Modifier.weight(1f).fillMaxWidth()) {
                        CompositionLocalProvider(LocalPageInsets provides insets) {
                            AnimatedContent(
                                targetState = nav.current,
                                transitionSpec = { fadeIn() togetherWith fadeOut() },
                                label = "page",
                            ) { s ->
                                Page(s, state, vm, nav)
                            }
                        }
                    }
                    if (!rail && onTab) {
                        NavigationBar(containerColor = MaterialTheme.colorScheme.surfaceContainer) {
                            for (t in tabs) {
                                NavigationBarItem(
                                    selected = nav.tab == t.screen,
                                    onClick = { nav.go(t.screen) },
                                    icon = { Icon(if (nav.tab == t.screen) t.selectedIcon else t.icon, null) },
                                    label = { Text(stringResource(t.label)) },
                                )
                            }
                        }
                    }
                }
            }
            SnackbarHost(
                snackbar,
                Modifier.align(Alignment.BottomCenter).safeDrawingPadding().padding(bottom = if (onTab && !rail) 80.dp else 16.dp),
            )
        }
    }

    state.prompts.firstOrNull()?.let { p -> PromptDialog(p) { vm.answer(p, it) } }

    state.conflict?.let { c ->
        AlertDialog(
            onDismissRequest = { vm.resolveConflict(null) },
            title = { Text(stringResource(R.string.prompt_conflict_title, c.name)) },
            text = {
                Column {
                    Text(stringResource(R.string.prompt_conflict_keep))
                    Text(stringResource(R.string.conflict_this_device, describe(c.plan.local)), Modifier.padding(top = 12.dp))
                    Text(stringResource(R.string.conflict_cloud, describe(c.plan.remote)), Modifier.padding(top = 4.dp))
                }
            },
            // One column in the confirm slot: split over the two slots, the dialog spaces the buttons apart and
            // can cut the last one off.
            confirmButton = {
                Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                    AppButton(onClick = { vm.resolveConflict(Resolution.Local) }) { Text(stringResource(R.string.action_keep_local)) }
                    AppButton(onClick = { vm.resolveConflict(Resolution.Cloud) }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_keep_cloud)) }
                    AppButton(onClick = { vm.resolveConflict(null) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_decide_later)) }
                }
            },
        )
    }
}

@Composable
private fun Page(s: Screen, state: UiState, vm: AppViewModel, nav: Nav) {
    when (s) {
        Screen.Library -> LibraryScreen(state, vm, nav)
        Screen.Emulators -> EmulatorsScreen(state, vm, nav)
        Screen.Settings -> SettingsScreen(nav)
        is Screen.Game -> GameScreen(s.id, state, vm, nav)
        is Screen.Emulator -> EmulatorScreen(s.id, state, vm, nav)
        Screen.Cloud -> CloudScreen(nav)
        Screen.Trash -> TrashScreen(nav)
        is Screen.Backups -> BackupsScreen(s.key, state, nav)
        is Screen.FindArt -> FindArtScreen(s.id, s.kind, state, nav)
        Screen.AddRemote -> AddRemoteScreen(nav)
        Screen.AddGame -> AddGameScreen(state, vm, nav)
        is Screen.EditGame -> EditGameScreen(s.id, state, vm, nav)
        is Screen.Frontends -> FrontendsScreen(s.emulatorId, state, vm, nav)
    }
}
