package com.akfaiz.carryover.ui.library

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.needsAttention
import com.akfaiz.carryover.core.ready

/** The desktop's filters: all games, those ready to play here, those that need attention. */
internal enum class LibraryFilter(val label: Int) {
    All(R.string.filter_all),
    Ready(R.string.filter_ready),
    Attention(R.string.filter_attention),
}

/** What the library page lists for a search and a filter. */
internal data class LibraryList(
    /** The games that match both, in the library's order. */
    val shown: List<GameView>,
    /** How many games of the whole library each filter holds, whatever the search. */
    val counts: Map<LibraryFilter, Int>,
    /** As on the desktop: the filter chips only when there is something to filter for, or one is on. */
    val showFilters: Boolean,
)

internal fun libraryList(games: List<GameView>, emulators: List<EmulatorView>, query: String, filter: LibraryFilter): LibraryList {
    val ready = games.filter { it.ready(emulators) }.map { it.id }.toSet()
    val attention = games.filter { it.needsAttention(emulators) }.map { it.id }.toSet()
    val search = query.trim()
    val shown = games.filter { g ->
        g.name.contains(search, ignoreCase = true) &&
            when (filter) {
                LibraryFilter.All -> true
                LibraryFilter.Ready -> g.id in ready
                LibraryFilter.Attention -> g.id in attention
            }
    }
    return LibraryList(
        shown = shown,
        counts = mapOf(LibraryFilter.All to games.size, LibraryFilter.Ready to ready.size, LibraryFilter.Attention to attention.size),
        showFilters = attention.isNotEmpty() || ready.size < games.size || filter != LibraryFilter.All,
    )
}
