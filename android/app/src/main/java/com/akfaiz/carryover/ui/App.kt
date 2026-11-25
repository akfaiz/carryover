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
import androidx.compose.foundation.layout.height
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
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Answer
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.PromptKind
import com.akfaiz.carryover.core.Resolution

/** The app's pages: three tabs, and pages opened on top of them. */
sealed interface Screen {
    data object Library : Screen
    data object Emulators : Screen
    data object Settings : Screen
    data class Game(val id: String) : Screen
    data class Emulator(val id: String) : Screen
    data object Cloud : Screen
    data object Trash : Screen

    /** The backups of one profile of a game (a game id, or `<game>.<emulator>`). */
    data class Backups(val key: String) : Screen

    /** Finding a cover or a background for a game on SteamGridDB. */
    data class FindArt(val id: String, val kind: ArtKind) : Screen
    data object AddRemote : Screen
    data object AddGame : Screen
    data class EditGame(val id: String) : Screen

    /** How to start games from a frontend, with the settings made for an emulator ("" = the first). */
    data class Frontends(val emulatorId: String = "") : Screen
}

/** Where the user is: a tab, and the pages opened over it (the last one is shown). */
class Nav {
    var tab by mutableStateOf<Screen>(Screen.Library)
        private set
    val stack = mutableStateListOf<Screen>()

    /** Set by a page that needs the SteamGridDB key: Settings opens its dialog once and clears it. */
    var askGridKey by mutableStateOf(false)
    val current: Screen get() = stack.lastOrNull() ?: tab
    val canGoBack: Boolean get() = stack.isNotEmpty() || tab != Screen.Library

    fun go(s: Screen) {
        if (s == Screen.Library || s == Screen.Emulators || s == Screen.Settings) {
            stack.clear()
            tab = s
        } else {
            stack.add(s)
        }
    }

    fun back() {
        when {
            stack.isNotEmpty() -> stack.removeAt(stack.lastIndex)
            tab != Screen.Library -> tab = Screen.Library
        }
    }

    companion object {
        private fun encode(s: Screen): String = when (s) {
            Screen.Library -> "library"
            Screen.Emulators -> "emulators"
            Screen.Settings -> "settings"
            is Screen.Game -> "game:${s.id}"
            is Screen.Emulator -> "emulator:${s.id}"
            Screen.Cloud -> "cloud"
            Screen.Trash -> "trash"
            is Screen.Backups -> "backups:${s.key}"
            is Screen.FindArt -> "findArt:${s.id}:${s.kind.wire}"
            Screen.AddRemote -> "addRemote"
            Screen.AddGame -> "addGame"
            is Screen.EditGame -> "editGame:${s.id}"
            is Screen.Frontends -> "frontends:${s.emulatorId}"
        }

        private fun decode(v: String): Screen? = when {
            v == "library" -> Screen.Library
            v == "emulators" -> Screen.Emulators
            v == "settings" -> Screen.Settings
            v.startsWith("game:") -> Screen.Game(v.removePrefix("game:"))
            v.startsWith("emulator:") -> Screen.Emulator(v.removePrefix("emulator:"))
            v == "cloud" -> Screen.Cloud
            v == "trash" -> Screen.Trash
            v.startsWith("backups:") -> Screen.Backups(v.removePrefix("backups:"))
            v.startsWith("findArt:") -> v.removePrefix("findArt:").let { rest ->
                val kind = ArtKind.entries.firstOrNull { it.wire == rest.substringAfterLast(':') } ?: ArtKind.Cover
                Screen.FindArt(rest.substringBeforeLast(':'), kind)
            }
            v == "addRemote" -> Screen.AddRemote
            v == "addGame" -> Screen.AddGame
            v.startsWith("editGame:") -> Screen.EditGame(v.removePrefix("editGame:"))
            v.startsWith("frontends:") -> Screen.Frontends(v.removePrefix("frontends:"))
            else -> null
        }

        /** Keeps the place across activity restarts (rotation, a new language). */
        val Saver: Saver<Nav, Any> = listSaver(
            save = { nav -> listOf(encode(nav.tab)) + nav.stack.map(::encode) },
            restore = { saved ->
                Nav().apply {
                    saved.mapNotNull(::decode).forEachIndexed { i, s -> if (i == 0) go(s) else stack.add(s) }
                }
            },
        )
    }
}

/**
 * The window insets a page's Scaffold pads its content by: the shell's
 * navigation bar or rail already keeps clear of the system bars on its side.
 */
val LocalPageInsets = staticCompositionLocalOf { WindowInsets(0, 0, 0, 0) }

/** A page's top bar insets: the shell's, so the rail's side isn't padded twice. */
@Composable
fun pageBarInsets(): WindowInsets = LocalPageInsets.current.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal)

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

/** A question from a play session (before or after playing); the session waits for the answer. */
@Composable
fun PromptDialog(p: Prompt, onAnswer: (String) -> Unit) {
    when (p.kind) {
        PromptKind.Conflict -> AlertDialog(
            onDismissRequest = {},
            title = { Text(stringResource(R.string.prompt_conflict_title, p.gameName)) },
            text = {
                Column {
                    Text(stringResource(if (p.beforePlay) R.string.prompt_conflict_play else R.string.prompt_conflict_keep))
                    p.conflict?.let {
                        Text(stringResource(R.string.conflict_this_device, describe(it.local)), Modifier.padding(top = 12.dp))
                        Text(stringResource(R.string.conflict_cloud, describe(it.remote)), Modifier.padding(top = 4.dp))
                    }
                }
            },
            confirmButton = {
                Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.End) {
                    AppButton(onClick = { onAnswer(Answer.Local.wire) }) { Text(stringResource(R.string.action_keep_local)) }
                    AppButton(onClick = { onAnswer(Answer.Cloud.wire) }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_keep_cloud)) }
                    // Skip leaves both sides as they are: before playing, the game starts with this device's saves.
                    AppButton(onClick = { onAnswer(Answer.Skip.wire) }, kind = BtnKind.Ghost) {
                        Text(stringResource(if (p.beforePlay) R.string.action_play_no_sync else R.string.action_decide_later))
                    }
                }
            },
        )
        PromptKind.Lock -> AlertDialog(
            onDismissRequest = {},
            title = {
                Text(
                    stringResource(
                        R.string.prompt_lock_title,
                        p.gameName,
                        p.lock?.deviceName?.ifEmpty { null } ?: stringResource(R.string.prompt_lock_another),
                    ),
                )
            },
            text = { Text(stringResource(R.string.prompt_lock_text)) },
            confirmButton = { AppButton(onClick = { onAnswer(Answer.Yes.wire) }) { Text(stringResource(R.string.action_play_anyway)) } },
            dismissButton = { AppButton(onClick = { onAnswer(Answer.No.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_dont_play)) } },
        )
        PromptKind.SyncError -> AlertDialog(
            onDismissRequest = {},
            title = { Text(stringResource(R.string.prompt_sync_error_title, p.gameName)) },
            text = {
                Column {
                    Text(p.error)
                    if (p.beforePlay) {
                        Spacer(Modifier.height(12.dp))
                        Text(stringResource(R.string.prompt_sync_error_play))
                    }
                }
            },
            confirmButton = {
                AppButton(onClick = { onAnswer(Answer.Yes.wire) }) {
                    Text(stringResource(if (p.beforePlay) R.string.action_play_anyway else R.string.action_ok))
                }
            },
            dismissButton = if (p.beforePlay) {
                { AppButton(onClick = { onAnswer(Answer.No.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_dont_play)) } }
            } else {
                null
            },
        )
        PromptKind.Emulator -> AlertDialog(
            onDismissRequest = { onAnswer(Answer.Cancel.wire) },
            title = { Text(stringResource(R.string.prompt_emulator_title, p.gameName)) },
            text = {
                Column {
                    for (c in p.choices) {
                        AppButton(onClick = { onAnswer(c.id) }, Modifier.fillMaxWidth(), BtnKind.Secondary) { Text(c.name) }
                    }
                }
            },
            confirmButton = {},
            dismissButton = { AppButton(onClick = { onAnswer(Answer.Cancel.wire) }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
        )
        PromptKind.Unknown -> onAnswer(Answer.No.wire)
    }
}
