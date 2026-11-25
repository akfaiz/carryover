package com.akfaiz.carryover.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame

/**
 * Finds a cover or a background for a game on SteamGridDB: search by name
 * (the game's own name is filled in), choose the game, then choose an image.
 * Without an API key there is only the way to set one.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FindArtScreen(id: String, kind: ArtKind, state: UiState, nav: Nav, vm: ArtViewModel = viewModel()) {
    val name = state.games.firstOrNull { it.id == id }?.name.orEmpty()
    LaunchedEffect(id, kind) { vm.open(id, name, kind) }
    val s by vm.state.collectAsStateWithLifecycle()
    // Back from the images of a game goes to the games found, not out of the page.
    BackHandler(enabled = s.picked != null) { vm.backToGames() }

    Scaffold(
        contentWindowInsets = LocalPageInsets.current,
        topBar = {
            PageTopBar(
                stringResource(if (kind == ArtKind.Cover) R.string.find_cover_title else R.string.find_background_title),
                { if (s.picked != null) vm.backToGames() else nav.back() },
            )
        },
    ) { padding ->
        Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.TopCenter) {
            if (!s.hasKey) {
                NeedKey(nav)
            } else {
                Column(Modifier.widthIn(max = 760.dp).fillMaxSize().padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    OutlinedTextField(
                        value = s.query,
                        onValueChange = vm::setQuery,
                        label = { Text(stringResource(R.string.find_search_label)) },
                        singleLine = true,
                        trailingIcon = {
                            IconButton(onClick = { vm.search(s.query) }) {
                                Icon(Icons.Outlined.Search, contentDescription = stringResource(R.string.action_search))
                            }
                        },
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                        keyboardActions = KeyboardActions(onSearch = { vm.search(s.query) }),
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Box(Modifier.weight(1f).fillMaxWidth()) {
                        val picked = s.picked
                        when {
                            s.searching || s.loadingArt -> Loading()
                            picked != null -> Images(kind, picked, s.grids.orEmpty(), enabled = !s.applying) { art -> vm.apply(art) { nav.back() } }
                            else -> Games(s.games) { vm.pick(it) }
                        }
                    }
                    Text(
                        stringResource(R.string.find_credit),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(bottom = 12.dp),
                    )
                }
            }
            if (s.applying) {
                Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.scrim.copy(alpha = 0.45f)), contentAlignment = Alignment.Center) {
                    SectionCard(null, Modifier.padding(32.dp).widthIn(max = 320.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            Spinner()
                            Text(stringResource(R.string.find_applying))
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun NeedKey(nav: Nav) {
    Callout(
        Tone.Info,
        Icons.Outlined.Info,
        stringResource(R.string.find_need_key),
        Modifier.widthIn(max = 760.dp).padding(16.dp),
        actions = {
            AppButton(onClick = {
                nav.askGridKey = true
                nav.go(Screen.Settings)
            }) { Text(stringResource(R.string.action_open_settings)) }
        },
    )
}

@Composable
private fun Loading() {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Spinner() }
}

/** The SteamGridDB games found (null before the first search). */
@Composable
private fun Games(games: List<GridGame>?, onPick: (GridGame) -> Unit) {
    when {
        games == null -> {}
        games.isEmpty() -> Hint(stringResource(R.string.find_no_games))
        else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(games, key = { it.id }) { g ->
                val shape = MaterialTheme.shapes.medium
                SectionCard(null, Modifier.focusRing(shape).clip(shape).clickable { onPick(g) }, gap = 4.dp) {
                    Text(g.name, style = MaterialTheme.typography.bodyLarge)
                    val detail = listOfNotNull(
                        g.year.takeIf { it > 0 }?.toString(),
                        stringResource(R.string.find_verified).takeIf { g.verified },
                    ).joinToString(" · ")
                    if (detail.isNotEmpty()) Hint(detail)
                }
            }
        }
    }
}

/** The images of a game: covers three across, backgrounds one under another. */
@Composable
private fun Images(kind: ArtKind, game: GridGame, grids: List<GridArt>, enabled: Boolean, onChoose: (GridArt) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(game.name, style = MaterialTheme.typography.titleSmall)
        if (grids.isEmpty()) {
            Hint(stringResource(R.string.find_no_art))
            return@Column
        }
        LazyVerticalGrid(
            columns = GridCells.Fixed(if (kind == ArtKind.Cover) 3 else 1),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(grids, key = { it.id }) { art ->
                val ratio = if (art.width > 0 && art.height > 0) art.width.toFloat() / art.height else if (kind == ArtKind.Cover) 2f / 3f else 3f
                Column(Modifier.focusRing(MaterialTheme.shapes.small).clip(MaterialTheme.shapes.small).clickable(enabled = enabled) { onChoose(art) }) {
                    RemoteImage(art.thumb, Modifier.fillMaxWidth().aspectRatio(ratio).clip(MaterialTheme.shapes.small))
                    if (art.author.isNotEmpty()) {
                        Text(
                            stringResource(R.string.find_art_by, art.author),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 1,
                            modifier = Modifier.padding(top = 2.dp),
                        )
                    }
                }
            }
        }
    }
}

/** A SteamGridDB image fetched from the network; a grey box while it loads (or when it can't). */
@Composable
private fun RemoteImage(url: String, modifier: Modifier) {
    val image = rememberRemoteImage(url, maxWidth = 600)
    Box(modifier.background(MaterialTheme.colorScheme.surfaceVariant), contentAlignment = Alignment.Center) {
        if (image != null) Image(image, contentDescription = null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
    }
}
