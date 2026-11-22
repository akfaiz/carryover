package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.ArtCore
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/** Which of SteamGridDB's answers to take when nobody chooses. */
object ArtPicks {
    /** A name as letters and digits only, in lower case: "Zelda: BotW" and "zelda botw" are the same. */
    fun normalize(name: String): String = name.lowercase().filter { it.isLetterOrDigit() }

    /**
     * The SteamGridDB game for a game's [name]: the one with the same name,
     * else the first that SteamGridDB verified, else the first (it ranks by
     * how well the name fits). Null when there are none.
     */
    fun bestGame(name: String, found: List<GridGame>): GridGame? {
        if (found.isEmpty()) return null
        val wanted = normalize(name)
        return found.firstOrNull { normalize(it.name) == wanted } ?: found.firstOrNull { it.verified } ?: found.first()
    }

    /** The best image: the highest score, then the largest. Null when there are none. */
    fun bestArt(grids: List<GridArt>): GridArt? =
        grids.maxWithOrNull(compareBy<GridArt>({ it.score }, { it.width.toLong() * it.height }))
}

/** How far a search for covers is. */
data class FindProgress(val done: Int, val total: Int, val name: String, val found: Int)

/** How a search for the covers of many games went. */
sealed interface FindReport {
    /** Every game already has a cover. */
    data object NothingToDo : FindReport

    /** It went through all [total] games and found a cover for [found] of them. */
    data class Done(val found: Int, val total: Int) : FindReport

    /** It stopped after repeated failures (no network, a rejected key), with [found] covers of [total] games by then. */
    data class Failed(val found: Int, val total: Int, val cause: Exception) : FindReport
}

/**
 * Finds covers on SteamGridDB for the games that have none, one after the
 * other (a game with no match is skipped), and for one game. No Android
 * classes: unit tested with a fake [ArtCore].
 */
class CoverFinder(private val core: ArtCore) {
    private val _state = MutableStateFlow<FindProgress?>(null)

    /** The search running now; null when there is none. */
    val state: StateFlow<FindProgress?> = _state

    /** Finds a cover for each of [games] without one. Null when a search is already running. */
    suspend fun findMissing(games: List<GameView>): FindReport? {
        val missing = games.filter { it.coverPath.isEmpty() }
        if (missing.isEmpty()) return FindReport.NothingToDo
        if (!_state.compareAndSet(null, FindProgress(0, missing.size, missing.first().name, 0))) return null
        var found = 0
        var failures = 0
        try {
            for ((i, game) in missing.withIndex()) {
                _state.value = FindProgress(i, missing.size, game.name, found)
                try {
                    if (findCover(game)) found++
                    failures = 0
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    // One game can fail alone; a second failure in a row is the network or the key.
                    if (++failures >= MAX_FAILURES) return FindReport.Failed(found, missing.size, e)
                }
            }
            return FindReport.Done(found, missing.size)
        } finally {
            _state.value = null
        }
    }

    /** Makes the best cover of SteamGridDB the cover of [game]; false when SteamGridDB has none. */
    suspend fun findCover(game: GameView): Boolean {
        val match = ArtPicks.bestGame(game.name, core.searchGridGames(game.name)) ?: return false
        val art = ArtPicks.bestArt(core.gridArt(ArtKind.Cover, match.id)) ?: return false
        core.setCoverFromUrl(game.id, art.url)
        return true
    }

    private companion object {
        const val MAX_FAILURES = 2
    }
}
