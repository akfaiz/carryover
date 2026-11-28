package com.akfaiz.carryover.ui.art

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame
import com.akfaiz.carryover.data.AppServices
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the page that finds a cover or a background shows. */
data class ArtState(
    val gameId: String = "",
    val kind: ArtKind = ArtKind.Cover,
    /** A SteamGridDB API key is set (without one there is nothing to search). */
    val hasKey: Boolean = true,
    val query: String = "",
    val searching: Boolean = false,
    /** The SteamGridDB games found; null before the first search. */
    val games: List<GridGame>? = null,
    /** The game whose artwork is shown. */
    val picked: GridGame? = null,
    val loadingArt: Boolean = false,
    /** The artwork of [picked]; null until it is loaded. */
    val grids: List<GridArt>? = null,
    /** The chosen image is being downloaded. */
    val applying: Boolean = false,
)

/**
 * Finding a cover or a background for one game on SteamGridDB: search by name,
 * choose the game, choose an image, which the core downloads and keeps. The
 * view model outlives the page, so [open] starts it afresh each time.
 */
class ArtViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {
    private val core get() = services.core
    private val m get() = services.messenger

    private val _state = MutableStateFlow(ArtState())
    val state: StateFlow<ArtState> = _state

    /** Answers of an older search or choice that arrive late are dropped. */
    private var token = 0

    /** Starts finding artwork of [kind] for game [gameId], searching for [name] at once. */
    fun open(gameId: String, name: String, kind: ArtKind) {
        token++
        val hasKey = services.settings.state.value.hasGridKey
        _state.value = ArtState(gameId = gameId, kind = kind, hasKey = hasKey, query = name)
        if (hasKey && name.isNotBlank()) search(name)
    }

    fun setQuery(query: String) = _state.update { it.copy(query = query) }

    /** Searches SteamGridDB for games named like [term]. */
    fun search(term: String) {
        if (term.isBlank()) return
        val mine = ++token
        _state.update { it.copy(query = term, searching = true, picked = null, grids = null, loadingArt = false) }
        viewModelScope.launch {
            val found = m.attempt(m.str(R.string.err_find_art)) { core.searchGridGames(term.trim()) }
            if (mine != token) return@launch
            _state.update { it.copy(searching = false, games = found ?: it.games) }
        }
    }

    /** Shows the artwork of [game]. */
    fun pick(game: GridGame) {
        val mine = ++token
        val kind = _state.value.kind
        _state.update { it.copy(picked = game, grids = null, loadingArt = true) }
        viewModelScope.launch {
            val art = m.attempt(m.str(R.string.err_find_art)) { core.gridArt(kind, game.id) }
            if (mine != token) return@launch
            _state.update { it.copy(loadingArt = false, grids = art, picked = if (art == null) null else it.picked) }
        }
    }

    /** Back from the artwork of a game to the games found. */
    fun backToGames() {
        token++
        _state.update { it.copy(picked = null, grids = null, loadingArt = false) }
    }

    /** Makes [art] the cover or the background of the game; [done] when it did. */
    fun apply(art: GridArt, done: () -> Unit) {
        val s = _state.value
        if (s.applying) return
        _state.update { it.copy(applying = true) }
        viewModelScope.launch {
            try {
                val ok = m.attempt(m.str(R.string.err_apply_art)) {
                    when (s.kind) {
                        ArtKind.Cover -> core.setCoverFromUrl(s.gameId, art.url)
                        ArtKind.Background -> core.setBackgroundFromUrl(s.gameId, art.url)
                    }
                }
                if (ok != null) {
                    m.say(m.str(if (s.kind == ArtKind.Cover) R.string.msg_cover_changed else R.string.msg_background_changed))
                    done()
                }
            } finally {
                _state.update { it.copy(applying = false) }
            }
        }
    }
}
