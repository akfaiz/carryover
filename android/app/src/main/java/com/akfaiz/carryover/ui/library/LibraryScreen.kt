package com.akfaiz.carryover.ui.library

import android.os.Build
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.ImageSearch
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.QueryStats
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Storage
import com.akfaiz.carryover.core.SyncAllProgress
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.core.appInfoIntent
import com.akfaiz.carryover.data.FindProgress
import com.akfaiz.carryover.ui.components.CoverImage
import com.akfaiz.carryover.ui.components.Permissions
import com.akfaiz.carryover.ui.components.StatusDot
import com.akfaiz.carryover.ui.components.color
import com.akfaiz.carryover.ui.components.label
import com.akfaiz.carryover.ui.components.relativeTime
import com.akfaiz.carryover.ui.components.rememberPermissions
import com.akfaiz.carryover.ui.kit.AppButton
import com.akfaiz.carryover.ui.kit.BtnKind
import com.akfaiz.carryover.ui.kit.EmptyState
import com.akfaiz.carryover.ui.kit.Hint
import com.akfaiz.carryover.ui.kit.LocalPageInsets
import com.akfaiz.carryover.ui.kit.PageTopBar
import com.akfaiz.carryover.ui.kit.SectionCard
import com.akfaiz.carryover.ui.kit.dpadExit
import com.akfaiz.carryover.ui.kit.focusRing
import com.akfaiz.carryover.ui.nav.Nav
import com.akfaiz.carryover.ui.nav.Screen
import com.akfaiz.carryover.ui.state.AppViewModel
import com.akfaiz.carryover.ui.state.Badge
import com.akfaiz.carryover.ui.state.UiState
import com.akfaiz.carryover.ui.state.badgeOf

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LibraryScreen(state: UiState, vm: AppViewModel, nav: Nav) {
    var query by rememberSaveable { mutableStateOf("") }
    var filter by rememberSaveable { mutableStateOf(LibraryFilter.All) }
    val ending = state.ending
    val badges = state.games.associate { it.id to badgeOf(it, state, ending) }
    val list = libraryList(state.games, state.emulators, query, filter)

    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = {
            PageTopBar(
                stringResource(R.string.nav_library),
                actions = {
                    if (state.hasGridKey && state.games.any { it.coverPath.isEmpty() }) {
                        IconButton(onClick = vm::findCovers, enabled = state.findingCovers == null) {
                            Icon(Icons.Outlined.ImageSearch, contentDescription = stringResource(R.string.action_find_covers))
                        }
                    }
                    IconButton(onClick = vm::syncAll, enabled = state.cloudReady && state.syncAll == null) {
                        Icon(Icons.Outlined.Sync, contentDescription = stringResource(R.string.action_sync_all))
                    }
                },
            )
        },
        floatingActionButton = {
            // With no games the empty page has the button already.
            if (!state.loaded || state.games.isNotEmpty()) ExtendedFloatingActionButton(
                onClick = { nav.go(Screen.AddGame) },
                icon = { Icon(Icons.Outlined.Add, contentDescription = null) },
                text = { Text(stringResource(R.string.library_add_game)) },
            )
        },
    ) { padding ->
        // Pulling brings in what other devices added to the library; the saves of every game are Sync all's (the toolbar).
        PullToRefreshBox(
            isRefreshing = state.refreshingLibrary,
            onRefresh = { if (state.cloudReady) vm.refreshLibrary() },
            modifier = Modifier.fillMaxSize().padding(padding),
        ) {
            LazyVerticalGrid(
                // Three covers across a phone, more on wider screens.
                columns = GridCells.Adaptive(minSize = 104.dp),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 96.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                item(span = { GridItemSpan(maxLineSpan) }) { SetupCard(state, vm, nav) }
                state.findingCovers?.let { p ->
                    item(span = { GridItemSpan(maxLineSpan) }) { FindCoversBar(p) }
                }
                state.syncAll?.let { p ->
                    item(span = { GridItemSpan(maxLineSpan) }) { SyncAllBar(p, state.syncAllEvent) }
                }
                if (state.games.isNotEmpty()) {
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            SearchField(query) { query = it }
                            // Scrolls sideways when the labels are long (Bahasa Indonesia on a phone).
                            if (list.showFilters) {
                                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    for (f in LibraryFilter.entries) {
                                        FilterChip(
                                            selected = filter == f,
                                            onClick = { filter = f },
                                            label = { Text(stringResource(f.label, list.counts.getValue(f)), maxLines = 1) },
                                            shape = MaterialTheme.shapes.extraLarge,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
                if (state.loaded && state.games.isEmpty()) {
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        EmptyState(
                            Icons.Outlined.VideoLibrary,
                            stringResource(R.string.library_empty_title),
                            stringResource(R.string.library_empty_text),
                            actions = {
                                AppButton(onClick = { nav.go(Screen.AddGame) }, icon = Icons.Outlined.Add) {
                                    Text(stringResource(R.string.library_add_first))
                                }
                            },
                        )
                    }
                } else if (state.loaded && list.shown.isEmpty()) {
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        Column(
                            Modifier.fillMaxWidth().padding(vertical = 24.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            Hint(
                                stringResource(
                                    when {
                                        query.isNotBlank() -> R.string.library_no_match
                                        filter == LibraryFilter.Ready -> R.string.library_none_ready
                                        else -> R.string.library_none_attention
                                    },
                                ),
                            )
                            if (query.isNotBlank()) {
                                AppButton(onClick = { query = "" }, kind = BtnKind.Secondary) { Text(stringResource(R.string.library_clear_search)) }
                            } else {
                                AppButton(onClick = { filter = LibraryFilter.All }, kind = BtnKind.Secondary) { Text(stringResource(R.string.library_show_all)) }
                            }
                        }
                    }
                }
                items(list.shown, key = { it.id }) { g ->
                    GameTile(g, badges[g.id] ?: Badge.NotSetUp) { nav.go(Screen.Game(g.id)) }
                }
            }
        }
    }
}

/**
 * How far Sync all is. The library comes first, with no count to show: the bar
 * runs without one. Then each game counts as one step, filled in as its
 * transfer goes, so the bar moves even with a single game.
 */
@Composable
private fun SyncAllBar(p: SyncAllProgress, ev: SyncEvent?) {
    val games = p.total > 0 && p.name.isNotEmpty()
    Column {
        Text(
            if (games) {
                stringResource(R.string.library_syncing_game, p.name, minOf(p.done + 1, p.total), p.total)
            } else {
                stringResource(R.string.library_syncing_library)
            },
            style = MaterialTheme.typography.bodyMedium,
        )
        if (!games) {
            LinearProgressIndicator(Modifier.fillMaxWidth().padding(top = 6.dp))
            return@Column
        }
        val inGame = when {
            p.done >= p.total || ev == null -> 0f
            ev.totalBytes > 0 -> (ev.bytes.toFloat() / ev.totalBytes).coerceIn(0f, 1f)
            ev.phase == "done" -> 1f
            ev.phase == "upload" || ev.phase == "download" -> 0.5f
            else -> 0.1f
        }
        val shown by animateFloatAsState((p.done + inGame) / p.total, label = "syncAll")
        LinearProgressIndicator(progress = { shown }, modifier = Modifier.fillMaxWidth().padding(top = 6.dp))
    }
}

@Composable
private fun SearchField(query: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = query,
        onValueChange = onChange,
        placeholder = { Text(stringResource(R.string.library_search)) },
        leadingIcon = { Icon(Icons.Outlined.Search, contentDescription = null) },
        trailingIcon = if (query.isNotEmpty()) {
            {
                IconButton(onClick = { onChange("") }) {
                    Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.action_clear))
                }
            }
        } else {
            null
        },
        singleLine = true,
        shape = MaterialTheme.shapes.small,
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        colors = TextFieldDefaults.colors(
            unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            unfocusedIndicatorColor = MaterialTheme.colorScheme.outline,
        ),
        modifier = Modifier.fillMaxWidth().dpadExit(),
    )
}

/** A game as a cover with its name and how its saves stand. */
@Composable
private fun GameTile(g: GameView, badge: Badge, open: () -> Unit) {
    val shape = MaterialTheme.shapes.medium
    Column(
        Modifier.fillMaxWidth().focusRing(shape).clip(shape).clickable(onClick = open).padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(MaterialTheme.shapes.small)) {
            CoverImage(g, Modifier.fillMaxSize())
            // Only states that ask for something; the others are the line below.
            if (badge < Badge.Synced) {
                StatusDot(badge, Modifier.align(Alignment.TopStart).padding(6.dp))
            }
        }
        Text(
            g.name,
            style = MaterialTheme.typography.titleSmall,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            tileLine(g, badge),
            style = MaterialTheme.typography.bodySmall,
            color = if (badge == Badge.Synced) MaterialTheme.colorScheme.onSurfaceVariant else badge.color(),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun tileLine(g: GameView, badge: Badge): String {
    if (badge != Badge.Synced) return badge.label()
    val last = g.profiles.map { it.status.lastSyncAt }.filter { it.isNotEmpty() }.maxOrNull().orEmpty()
    return stringResource(R.string.status_line_synced, relativeTime(last))
}

/**
 * What is left to set up, in one card: a cloud, access to save folders given
 * as paths, Usage access (to see a game was left) and notifications (results).
 * Each item only shows when it matters to this library.
 */
@Composable
private fun SetupCard(state: UiState, vm: AppViewModel, nav: Nav) {
    val ctx = LocalContext.current
    val perms = rememberPermissions()
    val pathFolders = state.games.any { g -> g.profiles.any { p -> p.resolvedPaths.values.any { it.startsWith("/") } } }
    val playable = state.games.any { g -> g.profiles.any { vm.launchSetup(it) != null } }
    val needCloud = state.loaded && !state.cloudReady
    val needStorage = pathFolders && !perms.storage
    val needUsage = playable && !perms.usage
    val needNotify = playable && Permissions.notificationsAsked && !perms.notifications
    if (!needCloud && !needStorage && !needUsage && !needNotify) return

    // With games to show, the card folds to one line (it is taller than a screen with every step open).
    val foldable = state.games.isNotEmpty()
    var open by rememberSaveable { mutableStateOf(false) }
    val showing = !foldable || open
    val steps = listOf(needCloud, needStorage, needUsage, needNotify).count { it }
    SectionCard(null) {
        val shape = MaterialTheme.shapes.small
        Row(
            Modifier.fillMaxWidth().then(if (foldable) Modifier.focusRing(shape).clip(shape).clickable { open = !open } else Modifier),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text(stringResource(R.string.setup_title), style = MaterialTheme.typography.titleMedium)
                if (!showing) Hint(pluralStringResource(R.plurals.setup_steps_left, steps, steps))
            }
            if (foldable) Icon(if (open) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore, null)
        }
        if (showing && needCloud) {
            SetupItem(Icons.Outlined.Cloud, R.string.setup_cloud_title, stringResource(R.string.setup_cloud_text)) {
                AppButton(onClick = { nav.go(Screen.Cloud) }) { Text(stringResource(R.string.setup_cloud_action)) }
            }
        }
        if (showing && needStorage) {
            SetupItem(
                Icons.Outlined.FolderOpen,
                R.string.setup_storage_title,
                stringResource(if (Storage.usesAllFilesAccess) R.string.setup_storage_text else R.string.setup_storage_text_legacy),
            ) {
                AppButton(onClick = perms.askStorage) { Text(stringResource(R.string.action_allow)) }
            }
        }
        if (showing && needUsage) {
            // Android 13+ greys the switch out ("Restricted setting") for an app installed
            // from a downloaded APK until it is unlocked in the app's own info page.
            val restricted = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
            SetupItem(
                Icons.Outlined.QueryStats,
                R.string.setup_usage_title,
                stringResource(R.string.setup_usage_text),
                note = if (restricted) stringResource(R.string.setup_usage_restricted) else null,
            ) {
                AppButton(onClick = perms.askUsage) { Text(stringResource(R.string.action_allow)) }
                if (restricted) {
                    AppButton(onClick = { ctx.startActivity(appInfoIntent(ctx)) }, kind = BtnKind.Secondary) {
                        Text(stringResource(R.string.action_app_info))
                    }
                }
            }
        }
        if (showing && needNotify) {
            SetupItem(Icons.Outlined.Notifications, R.string.setup_notifications_title, stringResource(R.string.setup_notifications_text)) {
                AppButton(onClick = perms.askNotifications) { Text(stringResource(R.string.action_allow)) }
            }
        }
    }
}

@Composable
private fun SetupItem(
    icon: ImageVector,
    title: Int,
    text: String,
    note: String? = null,
    actions: @Composable () -> Unit,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        // The empty state's tint: a soft circle with the accent icon.
        Surface(Modifier.size(40.dp), shape = CircleShape, color = MaterialTheme.colorScheme.primary.copy(alpha = 0.14f)) {
            Box(contentAlignment = Alignment.Center) {
                Icon(icon, null, Modifier.size(22.dp), tint = MaterialTheme.colorScheme.primary)
            }
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(stringResource(title), style = MaterialTheme.typography.titleSmall)
            Hint(text)
            note?.let { Hint(it) }
            Row(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) { actions() }
        }
    }
}

/** How far the search for covers is: the game it is on and a bar. */
@Composable
private fun FindCoversBar(p: FindProgress) {
    Column(Modifier.fillMaxWidth().padding(vertical = 4.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            stringResource(R.string.library_finding_covers, minOf(p.done + 1, p.total), p.total, p.name),
            style = MaterialTheme.typography.bodySmall,
            maxLines = 1,
        )
        LinearProgressIndicator(progress = { if (p.total == 0) 0f else p.done.toFloat() / p.total }, modifier = Modifier.fillMaxWidth())
    }
}
