package com.akfaiz.carryover.ui

import com.akfaiz.carryover.core.ArtKind
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Wallpaper
import com.akfaiz.carryover.core.TrashView
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material3.AlertDialog
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
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.automirrored.outlined.AddToHomeScreen
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.StopCircle
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
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
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.Resolution

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
                    Text(stringResource(R.string.game_no_profiles), color = MaterialTheme.colorScheme.onSurfaceVariant)
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
        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back))
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
                ) { Icon(Icons.Outlined.MoreVert, contentDescription = null) }
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
                            TextButton(
                                onClick = {
                                    deleting = false
                                    vm.deleteGame(g.id, g.name) { nav.back() }
                                },
                            ) { Text(stringResource(R.string.action_delete), color = MaterialTheme.colorScheme.error) }
                        },
                        dismissButton = { TextButton(onClick = { deleting = false }) { Text(stringResource(R.string.action_cancel)) } },
                    )
                }
            }
        }
        Row(
            Modifier.align(Alignment.BottomStart).widthIn(max = 760.dp).padding(horizontal = 16.dp),
            verticalAlignment = Alignment.Bottom,
            horizontalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            val shape = RoundedCornerShape(10.dp)
            Box(
                Modifier
                    .size(width = 104.dp, height = 156.dp)
                    .shadow(8.dp, shape)
                    .clip(shape)
                    .border(1.dp, MaterialTheme.colorScheme.outlineVariant, shape)
                    .clickable { pickCover.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) },
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
                StatusPill(badge)
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
            shape = RoundedCornerShape(14.dp),
            color = MaterialTheme.colorScheme.primaryContainer,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Filled.PlayArrow, null, tint = MaterialTheme.colorScheme.onPrimaryContainer)
                Text(
                    stringResource(R.string.status_playing) + (running.emulator?.let { " · $it" } ?: ""),
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.weight(1f).padding(start = 8.dp),
                )
                Button(onClick = { vm.donePlaying(running.key) }) {
                    Icon(Icons.Outlined.StopCircle, null, Modifier.size(18.dp))
                    Text(stringResource(R.string.action_done_playing), Modifier.padding(start = 6.dp))
                }
            }
        }
        starting != null -> Busy(state.sessions[starting.key]?.message ?: stringResource(R.string.status_checking))
        else -> {
            val playable = profiles.filter { vm.launchSetup(it) != null }
            val only = profiles.singleOrNull()
            if (playable.isEmpty() && only == null) return
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                if (playable.isNotEmpty()) PlayButton(g, playable, state, vm, Modifier.weight(1f))
                if (only != null && only.canSync) {
                    SyncButton(g, only, state, vm, if (playable.isEmpty()) Modifier.weight(1f) else Modifier)
                }
            }
        }
    }
}

@Composable
private fun Busy(text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        CircularProgressIndicator(Modifier.size(22.dp), strokeWidth = 2.dp)
        Text(text, style = MaterialTheme.typography.bodyMedium)
    }
}

@Composable
private fun PlayButton(g: GameView, playable: List<Profile>, state: UiState, vm: AppViewModel, modifier: Modifier) {
    var choosing by remember { mutableStateOf(false) }
    Box(modifier) {
        Button(
            onClick = { if (playable.size == 1) vm.play(g, playable[0]) else choosing = true },
            enabled = state.syncAll == null,
            modifier = Modifier.fillMaxWidth().height(52.dp),
            contentPadding = ButtonDefaults.ButtonWithIconContentPadding,
        ) {
            Icon(Icons.Filled.PlayArrow, null)
            Text(stringResource(R.string.action_play), Modifier.padding(start = 8.dp), style = MaterialTheme.typography.titleMedium)
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
                    leadingIcon = { Icon(Icons.Filled.PlayArrow, null) },
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
        OutlinedButton(onClick = { vm.cancelSync(p.key) }, modifier = modifier.height(52.dp)) {
            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
            Text(stringResource(R.string.action_cancel), Modifier.padding(start = 8.dp))
        }
        return
    }
    FilledTonalButton(
        onClick = { vm.sync(p.key, profileTitle(g, p)) },
        enabled = state.cloudReady && state.syncAll == null,
        modifier = modifier.height(52.dp),
    ) {
        Icon(Icons.Outlined.Sync, null, Modifier.size(18.dp))
        Text(stringResource(R.string.action_sync_now), Modifier.padding(start = 8.dp))
    }
}


/** Saves changed here and in the cloud: what each side has, and the choice. */
@Composable
private fun ConflictCard(g: GameView, p: Profile, vm: AppViewModel) {
    val plan by produceState<com.akfaiz.carryover.core.PlanView?>(null, p.key, p.status) { value = vm.plan(p.key) }
    val warn = LocalStatusColors.current.warn
    Card(
        colors = CardDefaults.cardColors(containerColor = warn.copy(alpha = 0.12f)),
        border = androidx.compose.foundation.BorderStroke(1.dp, warn.copy(alpha = 0.5f)),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Icon(Icons.Outlined.WarningAmber, null, tint = warn)
                Text(
                    stringResource(R.string.game_conflict_title) + (p.emulator?.let { " · $it" } ?: ""),
                    style = MaterialTheme.typography.titleMedium,
                )
            }
            Text(stringResource(R.string.game_conflict_text), style = MaterialTheme.typography.bodyMedium)
            plan?.let {
                Text(stringResource(R.string.conflict_this_device, describe(it.local)), style = MaterialTheme.typography.bodySmall)
                Text(stringResource(R.string.conflict_cloud, describe(it.remote)), style = MaterialTheme.typography.bodySmall)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(top = 4.dp)) {
                Button(onClick = { vm.sync(p.key, profileTitle(g, p), Resolution.Local) }) {
                    Text(stringResource(R.string.action_keep_local))
                }
                OutlinedButton(onClick = { vm.sync(p.key, profileTitle(g, p), Resolution.Cloud) }) {
                    Text(stringResource(R.string.action_keep_cloud))
                }
            }
        }
    }
}

/** One way the game is played here: its save folders, how it starts, and its own sync. */
@Composable
private fun ProfileCard(g: GameView, p: Profile, state: UiState, vm: AppViewModel, nav: Nav, ending: Set<String>) {
    // Edited paths by slot key, starting from the stored ones.
    val edits = remember(p.key, p.paths) { mutableStateMapOf<String, String>().apply { putAll(p.paths) } }
    val dirty = p.slots.any { (edits[it.key] ?: "") != (p.paths[it.key] ?: "") }
    val syncing = p.key in state.syncing
    val several = g.profiles.size > 1
    val suggest: SuggestViewModel = viewModel()
    val suggestState by suggest.state.collectAsStateWithLifecycle()
    if (suggestState.key == p.key) SaveSuggestDialog(suggestState, onUse = suggest::use, onClose = suggest::close)
    SectionCard(
        title = p.emulator ?: stringResource(R.string.game_profile_own),
        // With one profile, the header already shows its status.
        action = if (several) {
            { StatusPill(badgeOf(p, state, ending)) }
        } else {
            null
        },
    ) {
        Text(
            when {
                syncing -> state.syncing[p.key]?.message ?: stringResource(R.string.status_syncing)
                !p.canSync && p.pathErrors.isNotEmpty() -> p.pathErrors.values.first()
                !p.canSync -> stringResource(R.string.status_line_not_set_up)
                else -> statusLine(p.status)
            },
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (several && p.canSync) {
            SyncButton(g, p, state, vm)
        }

        Text(stringResource(R.string.game_save_folders), style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(top = 4.dp))
        for (slot in p.slots) {
            val err = p.pathErrors[slot.key]
            val resolved = p.resolvedPaths[slot.key]
            FolderField(
                value = edits[slot.key] ?: "",
                onChange = { edits[slot.key] = it },
                label = slot.title,
                supporting = err ?: resolved?.takeIf { it != edits[slot.key] }?.let { "→ $it" },
                isError = err != null,
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            AnimatedVisibility(dirty) {
                Button(onClick = { vm.setPaths(p.key, p.slots.associate { it.key to (edits[it.key] ?: "") }) }) {
                    Text(stringResource(R.string.action_save_paths))
                }
            }
            // Where the emulator keeps this game's saves, from its manifest (needs its type and data folder).
            if (state.emulators.firstOrNull { it.id == p.emulatorId }?.preset.orEmpty().isNotEmpty()) {
                TextButton(onClick = { suggest.open(p.key) }) {
                    Icon(Icons.Outlined.Search, null, Modifier.size(18.dp))
                    Text(stringResource(R.string.game_suggest_saves), Modifier.padding(start = 6.dp))
                }
            }
        }
        if (state.cloudReady) {
            TextButton(onClick = { nav.go(Screen.Backups(p.key)) }) {
                Icon(Icons.Outlined.History, null, Modifier.size(18.dp))
                Text(stringResource(R.string.game_backups), Modifier.padding(start = 6.dp))
            }
        }
        HorizontalDivider(Modifier.padding(vertical = 4.dp), color = MaterialTheme.colorScheme.outlineVariant)
        LaunchSection(g, p, state, vm, nav)
    }
}

/** Which app plays the profile on this device, and its game file. */
@Composable
private fun LaunchSection(g: GameView, p: Profile, state: UiState, vm: AppViewModel, nav: Nav) {
    val ctx = LocalContext.current
    var picking by rememberSaveable { mutableStateOf(false) }
    Text(stringResource(R.string.game_launch), style = MaterialTheme.typography.labelLarge)
    if (p.emulatorId == null) {
        AppLine(
            p.app.substringBefore('/'),
            if (p.app.isBlank()) stringResource(R.string.game_app_none) else stringResource(R.string.game_app, appLabel(ctx, p.app)),
        )
        OutlinedButton(onClick = { picking = true }) {
            Text(stringResource(if (p.app.isBlank()) R.string.action_choose_game_app else R.string.action_change_app))
        }
    } else {
        val emu = state.emulators.firstOrNull { it.id == p.emulatorId }
        val exe = emu?.device?.exe.orEmpty()
        AppLine(
            exe.substringBefore('/'),
            if (exe.isBlank()) {
                stringResource(R.string.game_emulator_missing, p.emulator.orEmpty())
            } else {
                stringResource(R.string.game_emulator_app, p.emulator.orEmpty(), appLabel(ctx, exe))
            },
        )
        if (exe.isBlank() || emu == null) {
            OutlinedButton(onClick = { nav.go(if (emu != null) Screen.Emulator(emu.id) else Screen.Emulators) }) {
                Text(stringResource(R.string.action_set_up_emulators))
            }
        } else if (emu.device.args.isNotBlank()) {
            var rom by remember(p.key, p.rom) { mutableStateOf(p.rom) }
            GameFileField(rom, { rom = it }, stringResource(R.string.game_file))
            AnimatedVisibility(rom != p.rom) {
                Button(onClick = { vm.setRom(p.key, rom) }) { Text(stringResource(R.string.action_save_game_file)) }
            }
        }
    }
    if (vm.launchSetup(p) != null) {
        TextButton(onClick = { vm.addShortcut(g, p) }) {
            Icon(Icons.AutoMirrored.Outlined.AddToHomeScreen, null, Modifier.size(18.dp))
            Text(stringResource(R.string.action_add_shortcut), Modifier.padding(start = 6.dp))
        }
    }
    if (picking) {
        AppPickerDialog(stringResource(R.string.game_choose_app_title, g.name), onDismiss = { picking = false }) { app ->
            picking = false
            vm.setNativeApp(g.id, app.pkg)
        }
    }
}

/** An app's icon (when installed) and a line about it. */
@Composable
private fun AppLine(pkg: String, text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        val icon = rememberAppIcon(pkg)
        if (icon != null) {
            Image(icon, null, Modifier.size(28.dp))
        } else {
            Icon(Icons.Outlined.Apps, null, Modifier.size(28.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(text, style = MaterialTheme.typography.bodyMedium)
    }
}

@Composable
private fun OtherDevices(g: GameView) {
    var open by rememberSaveable { mutableStateOf(false) }
    SectionCard(null) {
        Row(
            Modifier.fillMaxWidth().focusRing(RoundedCornerShape(8.dp)).clip(RoundedCornerShape(8.dp)).clickable { open = !open },
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
                            Text(
                                "${slot.title}: $it",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
        }
    }
}
