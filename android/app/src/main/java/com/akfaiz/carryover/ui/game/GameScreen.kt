package com.akfaiz.carryover.ui.game

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.AddToHomeScreen
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.StopCircle
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.Wallpaper
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.ui.components.coreText
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.core.TrashView
import com.akfaiz.carryover.ui.components.CoverImage
import com.akfaiz.carryover.ui.components.Placeholder
import com.akfaiz.carryover.ui.components.StatusBadge
import com.akfaiz.carryover.ui.components.describe
import com.akfaiz.carryover.ui.components.folderLocation
import com.akfaiz.carryover.ui.components.headerArt
import com.akfaiz.carryover.ui.components.rememberDocumentName
import com.akfaiz.carryover.ui.components.statusLine
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.Callout
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.PlayIcon
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.Spinner
import com.akfaiz.carryover.ui.kit.SubCard
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.kit.color
import com.akfaiz.carryover.ui.kit.focusRing
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.Badge
import com.akfaiz.carryover.ui.state.UiState
import com.akfaiz.carryover.ui.state.badgeOf
import com.akfaiz.carryover.ui.state.isContentUri
import com.akfaiz.carryover.ui.state.profileTitle

@Composable
fun GameScreen(id: String, state: UiState, vm: AppViewModel, nav: Nav) {
    val g = state.games.firstOrNull { it.id == id }
    val ending = state.ending
    Scaffold(contentWindowInsets = WindowInsets(0, 0, 0, 0)) { padding ->
        if (g == null) {
            Column(Modifier.padding(padding).statusBarsPadding()) {
                BackButton(nav, Modifier.padding(8.dp))
                Text(stringResource(R.string.game_missing), Modifier.padding(16.dp))
            }
            return@Scaffold
        }
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
        ) {
            Header(g, badgeOf(g, state, ending), vm, nav)
            Column(
                Modifier
                    .align(Alignment.CenterHorizontally)
                    .widthIn(max = 760.dp)
                    .padding(horizontal = 16.dp)
                    .padding(top = 20.dp)
                    .navigationBarsPadding()
                    .padding(bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                val profiles = g.profiles
                PlayRow(g, profiles, state, vm, ending)
                for (p in profiles) {
                    if (p.status.conflict && p.key !in state.syncing && p.canSync) ConflictCard(g, p, vm)
                }
                if (profiles.isEmpty()) {
                    Hint(stringResource(R.string.game_no_profiles))
                    AppButton(onClick = { nav.go(Screen.EditGame(g.id)) }, kind = BtnKind.Secondary, icon = Icons.Outlined.Edit) {
                        Text(stringResource(R.string.edit_game))
                    }
                }
                for (p in profiles) {
                    ProfileCard(g, p, state, vm, nav, ending)
                }
                if (g.otherDevices.isNotEmpty() && g.slots.isNotEmpty()) {
                    OtherDevices(g)
                }
            }
        }
    }
}

@Composable
private fun BackButton(nav: Nav, modifier: Modifier = Modifier) {
    IconButton(
        onClick = nav::back,
        modifier = modifier.focusRing(CircleShape),
        colors = IconButtonDefaults.iconButtonColors(
            containerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.75f),
            contentColor = MaterialTheme.colorScheme.onSurface,
        ),
    ) {
        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.action_back))
    }
}

/** Artwork across the top, the cover, the name and how the saves stand. */
@Composable
private fun Header(g: GameView, badge: Badge, vm: AppViewModel, nav: Nav) {
    val window = LocalWindowInfo.current.containerSize
    val width = window.width
    val art = headerArt(g, width)
    var menu by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf(false) }
    val pickCover = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri != null) vm.setCover(g.id, uri.toString())
    }
    val bg = MaterialTheme.colorScheme.background
    // Shorter on a landscape phone or handheld, so the actions show without scrolling.
    val tall = with(LocalDensity.current) { window.height.toDp() } >= 560.dp
    Box(Modifier.fillMaxWidth().height(if (tall) 300.dp else 220.dp)) {
        if (art != null) {
            Image(art, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop, alpha = 0.85f)
        } else {
            Placeholder("", Modifier.fillMaxSize()) // colour only, the name is below
        }
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0f to Color.Transparent, 0.45f to bg.copy(alpha = 0.35f), 1f to bg)))
        Row(Modifier.fillMaxWidth().statusBarsPadding().padding(8.dp)) {
            BackButton(nav)
            Box(Modifier.weight(1f))
            Box {
                IconButton(
                    onClick = { menu = true },
                    modifier = Modifier.focusRing(CircleShape),
                    colors = IconButtonDefaults.iconButtonColors(
                        containerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.75f),
                        contentColor = MaterialTheme.colorScheme.onSurface,
                    ),
                ) { Icon(Icons.Outlined.MoreVert, contentDescription = stringResource(R.string.action_more_options)) }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.edit_game)) },
                        leadingIcon = { Icon(Icons.Outlined.Edit, null) },
                        onClick = {
                            menu = false
                            nav.go(Screen.EditGame(g.id))
                        },
                    )
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.action_change_cover)) },
                        leadingIcon = { Icon(Icons.Outlined.Image, null) },
                        onClick = {
                            menu = false
                            pickCover.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly))
                        },
                    )
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.action_find_cover)) },
                        leadingIcon = { Icon(Icons.Outlined.Search, null) },
                        onClick = {
                            menu = false
                            nav.go(Screen.FindArt(g.id, ArtKind.Cover))
                        },
                    )
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.action_find_background)) },
                        leadingIcon = { Icon(Icons.Outlined.Wallpaper, null) },
                        onClick = {
                            menu = false
                            nav.go(Screen.FindArt(g.id, ArtKind.Background))
                        },
                    )
                    if (g.art["hero"] != null) {
                        DropdownMenuItem(
                            text = { Text(stringResource(R.string.action_remove_background)) },
                            leadingIcon = { Icon(Icons.Outlined.Close, null) },
                            onClick = {
                                menu = false
                                vm.clearBackground(g.id)
                            },
                        )
                    }
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.action_delete_game), color = MaterialTheme.colorScheme.error) },
                        leadingIcon = { Icon(Icons.Outlined.Delete, null, tint = MaterialTheme.colorScheme.error) },
                        onClick = {
                            menu = false
                            deleting = true
                        },
                    )
                }
                if (deleting) {
                    AlertDialog(
                        onDismissRequest = { deleting = false },
                        title = { Text(stringResource(R.string.dialog_delete_game_title, g.name)) },
                        text = { Text(stringResource(R.string.dialog_delete_game_text, TrashView().days)) },
                        confirmButton = {
                            AppButton(
                                onClick = {
                                    deleting = false
                                    vm.deleteGame(g.id, g.name) { nav.back() }
                                },
                                kind = BtnKind.Danger,
                            ) { Text(stringResource(R.string.action_delete)) }
                        },
                        dismissButton = { AppButton(onClick = { deleting = false }, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) } },
                    )
                }
            }
        }
        Row(
            Modifier.align(Alignment.BottomCenter).widthIn(max = 760.dp).fillMaxWidth().padding(horizontal = 16.dp),
            verticalAlignment = Alignment.Bottom,
            horizontalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            val shape = MaterialTheme.shapes.small
            Box(
                Modifier
                    .size(width = 104.dp, height = 156.dp)
                    .shadow(8.dp, shape)
                    .clip(shape)
                    // Not a button: a stray tap while scrolling shouldn't open the photo picker (the ⋮ menu changes it).
                    .border(1.dp, MaterialTheme.colorScheme.outlineVariant, shape),
            ) {
                CoverImage(g, Modifier.fillMaxSize())
            }
            Column(Modifier.weight(1f).padding(bottom = 4.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    g.name,
                    style = MaterialTheme.typography.headlineSmall,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
                StatusBadge(badge)
            }
        }
    }
}

/**
 * The main actions: Play (with a choice when the game is played in several
 * ways), Sync for a game with one way, or the session being played.
 */
@Composable
private fun PlayRow(g: GameView, profiles: List<Profile>, state: UiState, vm: AppViewModel, ending: Set<String>) {
    val running = profiles.firstOrNull { it.status.running }
    val starting = profiles.firstOrNull { it.key in state.starting }
    val uploading = profiles.firstOrNull { it.key in ending }
    when {
        uploading != null -> Busy(stringResource(R.string.status_uploading))
        running != null -> Surface(
            shape = MaterialTheme.shapes.medium,
            color = MaterialTheme.colorScheme.primaryContainer,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                Icon(PlayIcon, null, tint = MaterialTheme.colorScheme.onPrimaryContainer)
                Text(
                    stringResource(R.string.status_playing) + (running.emulator?.let { " · $it" } ?: ""),
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.weight(1f).padding(start = 8.dp),
                )
                AppButton(onClick = { vm.donePlaying(running.key) }, icon = Icons.Outlined.StopCircle) {
                    Text(stringResource(R.string.action_done_playing))
                }
            }
        }
        starting != null -> Busy(state.sessions[starting.key]?.let { coreText(it.message, it.msg) } ?: stringResource(R.string.status_checking))
        else -> {
            val playable = profiles.filter { vm.launchSetup(it) != null }
            val only = profiles.singleOrNull()
            if (playable.isEmpty() && only == null) return
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (playable.isNotEmpty()) PlayButton(g, playable, state, vm, Modifier.weight(1f))
                    if (only != null && only.canSync) {
                        SyncButton(g, only, state, vm, if (playable.isEmpty()) Modifier.weight(1f) else Modifier)
                    }
                }
                // Play waits for Sync all: say so rather than only greying it out.
                if (playable.isNotEmpty() && state.syncAll != null) {
                    Hint(stringResource(R.string.play_wait_sync_all))
                }
            }
        }
    }
}

@Composable
private fun Busy(text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Spinner()
        Text(text, style = MaterialTheme.typography.bodyMedium)
    }
}

@Composable
private fun PlayButton(g: GameView, playable: List<Profile>, state: UiState, vm: AppViewModel, modifier: Modifier) {
    var choosing by remember { mutableStateOf(false) }
    Box(modifier) {
        AppButton(
            onClick = { if (playable.size == 1) vm.play(g, playable[0]) else choosing = true },
            enabled = state.syncAll == null,
            modifier = Modifier.fillMaxWidth(),
            icon = PlayIcon,
            large = true,
        ) {
            Text(stringResource(R.string.action_play), style = MaterialTheme.typography.titleMedium)
        }
        DropdownMenu(expanded = choosing, onDismissRequest = { choosing = false }) {
            Text(
                stringResource(R.string.game_play_with),
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
            for (p in playable) {
                DropdownMenuItem(
                    text = { Text(p.emulator ?: stringResource(R.string.game_profile_own)) },
                    leadingIcon = { Icon(PlayIcon, null) },
                    onClick = {
                        choosing = false
                        vm.play(g, p)
                    },
                )
            }
        }
    }
}

@Composable
private fun SyncButton(g: GameView, p: Profile, state: UiState, vm: AppViewModel, modifier: Modifier = Modifier) {
    val syncing = p.key in state.syncing
    if (syncing) {
        AppButton(onClick = { vm.cancelSync(p.key) }, modifier, BtnKind.Secondary, large = true) {
            Spinner(small = true)
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.action_cancel))
        }
        return
    }
    AppButton(
        onClick = { vm.sync(p.key, profileTitle(g, p)) },
        modifier = modifier,
        kind = BtnKind.Secondary,
        icon = Icons.Outlined.Sync,
        enabled = state.cloudReady && state.syncAll == null,
        large = true,
    ) {
        Text(stringResource(R.string.action_sync_now))
    }
}


/** Saves changed here and in the cloud: what each side has, and the choice. */
@Composable
private fun ConflictCard(g: GameView, p: Profile, vm: AppViewModel) {
    val plan by produceState<com.akfaiz.carryover.core.PlanView?>(null, p.key, p.status) { value = vm.plan(p.key) }
    Callout(
        Tone.Err,
        Icons.Outlined.WarningAmber,
        stringResource(R.string.game_conflict_title) + (p.emulator?.let { " · $it" } ?: "") + "\n" + stringResource(R.string.game_conflict_text),
        content = {
            plan?.let {
                // What each side has, as on the desktop's two boxes.
                SubCard { Text(stringResource(R.string.conflict_this_device, describe(it.local)), style = MaterialTheme.typography.bodySmall) }
                SubCard { Text(stringResource(R.string.conflict_cloud, describe(it.remote)), style = MaterialTheme.typography.bodySmall) }
            }
        },
        actions = {
            AppButton(onClick = { vm.sync(p.key, profileTitle(g, p), Resolution.Local) }) { Text(stringResource(R.string.action_keep_local)) }
            AppButton(onClick = { vm.sync(p.key, profileTitle(g, p), Resolution.Cloud) }, kind = BtnKind.Secondary) {
                Text(stringResource(R.string.action_keep_cloud))
            }
        },
    )
}

/**
 * One way the game is played here, as it stands: its sync, where its saves are and which files of them sync, and
 * how it starts. Changing any of it is on the game's edit page.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ProfileCard(g: GameView, p: Profile, state: UiState, vm: AppViewModel, nav: Nav, ending: Set<String>) {
    val syncing = p.key in state.syncing
    val several = g.profiles.size > 1
    SectionCard(
        title = p.emulator ?: stringResource(R.string.game_profile_own),
        // With one profile, the header already shows its status.
        action = if (several) {
            { StatusBadge(badgeOf(p, state, ending)) }
        } else {
            null
        },
    ) {
        // What needs a look is a callout (as on the desktop); a quiet state is a line of grey text.
        val line = when {
            syncing -> state.syncing[p.key]?.let { coreText(it.message, it.msg) } ?: stringResource(R.string.status_syncing)
            !p.canSync && p.pathErrors.isNotEmpty() -> p.pathErrors.entries.first().let { coreText(it.value, p.pathErrorMsgs[it.key]) }
            !p.canSync -> stringResource(R.string.status_line_not_set_up)
            else -> statusLine(p.status)
        }
        when {
            syncing -> Hint(line)
            !p.canSync -> Callout(Tone.Warn, Icons.Outlined.WarningAmber, line)
            p.status.lastError.isNotEmpty() -> Callout(Tone.Err, Icons.Outlined.ErrorOutline, line)
            p.status.localChanges -> Callout(Tone.Warn, Icons.Outlined.CloudUpload, line)
            else -> Hint(line)
        }
        if (several && p.canSync) {
            SyncButton(g, p, state, vm)
        }
        Text(stringResource(R.string.game_save_folders), style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(top = 4.dp))
        for (slot in p.slots) {
            SaveFolderLine(slot, p)
        }
        Text(stringResource(R.string.game_launch), style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(top = 4.dp))
        ProfileAppLine(p, state)
        if (p.emulatorId != null && p.rom.isNotBlank()) {
            Hint(stringResource(R.string.game_file_line, if (isContentUri(p.rom)) rememberDocumentName(p.rom) else p.rom))
        }
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), itemVerticalAlignment = Alignment.CenterVertically) {
            // Not ready to play here yet: setting it up is the thing to do, so it is the button that stands out.
            if (p.ready(state.emulators)) {
                AppButton(onClick = { nav.go(Screen.EditGame(g.id)) }, kind = BtnKind.Ghost, icon = Icons.Outlined.Edit) {
                    Text(stringResource(R.string.action_edit))
                }
            } else {
                AppButton(onClick = { nav.go(Screen.EditGame(g.id)) }, icon = Icons.Outlined.Edit) {
                    Text(stringResource(R.string.action_set_up))
                }
            }
            if (state.cloudReady) {
                AppButton(onClick = { nav.go(Screen.Backups(p.key)) }, kind = BtnKind.Ghost, icon = Icons.Outlined.History) {
                    Text(stringResource(R.string.game_backups))
                }
            }
            if (vm.launchSetup(p) != null) {
                AppButton(onClick = { vm.addShortcut(g, p) }, kind = BtnKind.Ghost, icon = Icons.AutoMirrored.Outlined.AddToHomeScreen) {
                    Text(stringResource(R.string.action_add_shortcut))
                }
            }
        }
    }
}

/** A save location: its name, this device's folder for it (or that there is none yet), and which files sync. */
@Composable
private fun SaveFolderLine(slot: SaveSlot, p: Profile) {
    val value = p.paths[slot.key].orEmpty()
    val err = p.pathErrors[slot.key]?.takeIf { value.isNotBlank() }
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(slot.title, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(
            if (value.isBlank()) stringResource(R.string.game_folder_not_set) else folderLocation(value, p.resolvedPaths[slot.key]),
            style = MaterialTheme.typography.bodyMedium,
            color = if (value.isBlank()) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurface,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        err?.let { Text(coreText(it, p.pathErrorMsgs[slot.key]), style = MaterialTheme.typography.bodySmall, color = Tone.Err.color()) }
        filtersSummary(slot)?.let { Hint(it) }
    }
}

@Composable
private fun OtherDevices(g: GameView) {
    var open by rememberSaveable { mutableStateOf(false) }
    SectionCard(null) {
        Row(
            Modifier.fillMaxWidth().focusRing(MaterialTheme.shapes.small).clip(MaterialTheme.shapes.small).clickable { open = !open },
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Icon(Icons.Outlined.Devices, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(stringResource(R.string.game_other_devices), style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
            Icon(if (open) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore, null)
        }
        AnimatedVisibility(open) {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                for (d in g.otherDevices) {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    Text(d.deviceName, style = MaterialTheme.typography.labelLarge)
                    for (slot in g.slots) {
                        d.paths[slot.key]?.let {
                            Hint("${slot.title}: $it")
                        }
                    }
                }
            }
        }
    }
}
