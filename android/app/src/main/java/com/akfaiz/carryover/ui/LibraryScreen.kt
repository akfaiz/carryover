package com.akfaiz.carryover.ui

import com.akfaiz.carryover.data.FindProgress
import androidx.compose.material.icons.outlined.ImageSearch
import android.os.Build
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.QueryStats
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.needsAttention
import com.akfaiz.carryover.core.ready
import com.akfaiz.carryover.core.SyncAllProgress
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.core.Storage
import com.akfaiz.carryover.core.appInfoIntent

/** The desktop's filters: all games, those ready to play here, those that need attention. */
private enum class Filter(val label: Int) { All(R.string.filter_all), Ready(R.string.filter_ready), Attention(R.string.filter_attention) }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LibraryScreen(state: UiState, vm: AppViewModel, nav: Nav) {
    var query by rememberSaveable { mutableStateOf("") }
    var filter by rememberSaveable { mutableStateOf(Filter.All) }
    val ending = state.ending
    val badges = state.games.associate { it.id to badgeOf(it, state, ending) }
    val ready = state.games.filter { it.ready(state.emulators) }.map { it.id }.toSet()
    val attention = state.games.filter { it.needsAttention(state.emulators) }.map { it.id }.toSet()
    val counts = mapOf(Filter.All to state.games.size, Filter.Ready to ready.size, Filter.Attention to attention.size)
    // As on the desktop: the chips only when there is something to filter for, or one is on.
    val showFilters = attention.isNotEmpty() || ready.size < state.games.size || filter != Filter.All
    val shown = state.games.filter { g ->
        (query.isBlank() || g.name.contains(query.trim(), ignoreCase = true)) &&
            when (filter) {
                Filter.All -> true
                Filter.Ready -> g.id in ready
                Filter.Attention -> g.id in attention
            }
    }

    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.nav_library)) },
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
                windowInsets = pageBarInsets(),
            )
        },
        floatingActionButton = {
            ExtendedFloatingActionButton(
                onClick = { nav.go(Screen.AddGame) },
                icon = { Icon(Icons.Filled.Add, contentDescription = null) },
                text = { Text(stringResource(R.string.library_add_game)) },
            )
        },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.syncAll != null,
            onRefresh = { if (state.cloudReady) vm.syncAll() },
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
                            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                if (showFilters) {
                                    for (f in Filter.entries) {
                                        FilterChip(
                                            selected = filter == f,
                                            onClick = { filter = f },
                                            label = { Text(stringResource(f.label, counts.getValue(f)), maxLines = 1) },
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
                        )
                    }
                } else if (state.loaded && shown.isEmpty()) {
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        Text(
                            stringResource(
                                when {
                                    query.isNotBlank() -> R.string.library_no_match
                                    filter == Filter.Ready -> R.string.library_none_ready
                                    else -> R.string.library_none_attention
                                },
                            ),
                            Modifier.padding(vertical = 32.dp),
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
                items(shown, key = { it.id }) { g ->
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
        shape = CircleShape,
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        colors = TextFieldDefaults.colors(
            unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            unfocusedIndicatorColor = MaterialTheme.colorScheme.surfaceContainerHigh,
        ),
        modifier = Modifier.fillMaxWidth().dpadExit(),
    )
}

/** A game as a cover with its name and how its saves stand. */
@Composable
private fun GameTile(g: GameView, badge: Badge, open: () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Column(
        Modifier.fillMaxWidth().focusRing(shape).clip(shape).clickable(onClick = open).padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(10.dp))) {
            CoverImage(g, Modifier.fillMaxSize())
            // Only states that ask for something; the others are the line below.
            if (badge < Badge.Synced) {
                StatusPill(badge, Modifier.align(Alignment.TopStart).padding(6.dp), iconOnly = true)
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

    SectionCard(stringResource(R.string.setup_title)) {
        if (needCloud) {
            SetupItem(Icons.Outlined.Cloud, R.string.setup_cloud_title, stringResource(R.string.setup_cloud_text)) {
                Button(onClick = { nav.go(Screen.Cloud) }) { Text(stringResource(R.string.setup_cloud_action)) }
            }
        }
        if (needStorage) {
            SetupItem(
                Icons.Outlined.FolderOpen,
                R.string.setup_storage_title,
                stringResource(if (Storage.usesAllFilesAccess) R.string.setup_storage_text else R.string.setup_storage_text_legacy),
            ) {
                Button(onClick = perms.askStorage) { Text(stringResource(R.string.action_allow)) }
            }
        }
        if (needUsage) {
            // Android 13+ greys the switch out ("Restricted setting") for an app installed
            // from a downloaded APK until it is unlocked in the app's own info page.
            val restricted = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
            SetupItem(
                Icons.Outlined.QueryStats,
                R.string.setup_usage_title,
                stringResource(R.string.setup_usage_text),
                note = if (restricted) stringResource(R.string.setup_usage_restricted) else null,
            ) {
                Button(onClick = perms.askUsage) { Text(stringResource(R.string.action_allow)) }
                if (restricted) {
                    OutlinedButton(onClick = { ctx.startActivity(appInfoIntent(ctx)) }) {
                        Text(stringResource(R.string.action_app_info))
                    }
                }
            }
        }
        if (needNotify) {
            SetupItem(Icons.Outlined.Notifications, R.string.setup_notifications_title, stringResource(R.string.setup_notifications_text)) {
                Button(onClick = perms.askNotifications) { Text(stringResource(R.string.action_allow)) }
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
        Surface(Modifier.size(40.dp), shape = CircleShape, color = MaterialTheme.colorScheme.primaryContainer) {
            Box(contentAlignment = Alignment.Center) {
                Icon(icon, null, Modifier.size(22.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
            }
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(stringResource(title), style = MaterialTheme.typography.titleSmall)
            Text(text, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            note?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
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
