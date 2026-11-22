package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame
import com.akfaiz.carryover.ui.FakeArtCore
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ArtPicksTest {
    private fun game(id: Int, name: String, verified: Boolean = false) = GridGame(id = id, name = name, verified = verified)

    @Test
    fun theGameWithTheSameNameWinsWhateverTheCaseOrPunctuation() {
        val found = listOf(game(1, "Zelda 2", verified = true), game(2, "The Legend of Zelda: BotW"), game(3, "Zelda"))
        assertEquals(2, ArtPicks.bestGame("the legend of zelda botw", found)?.id)
        assertEquals(3, ArtPicks.bestGame("ZELDA", found)?.id)
    }

    @Test
    fun withoutAnExactNameTheFirstVerifiedWins() {
        val found = listOf(game(1, "Mario Kart"), game(2, "Mario Kart 8", verified = true), game(3, "Mario Kart 7", verified = true))
        assertEquals(2, ArtPicks.bestGame("Mario", found)?.id)
    }

    @Test
    fun withNoneVerifiedTheFirstWins() {
        assertEquals(7, ArtPicks.bestGame("x", listOf(game(7, "A"), game(8, "B")))?.id)
    }

    @Test
    fun noGamesIsNoMatch() {
        assertNull(ArtPicks.bestGame("x", emptyList()))
    }

    @Test
    fun theNamesAreComparedByLettersAndDigitsOnly() {
        assertEquals("supermarioworld", ArtPicks.normalize("Super Mario World!"))
        assertEquals("", ArtPicks.normalize(" - : "))
    }

    private fun art(id: Int, score: Int, w: Int = 600, h: Int = 900) = GridArt(id = id, url = "u$id", score = score, width = w, height = h)

    @Test
    fun theHighestScoreIsTheBestArt() {
        assertEquals(2, ArtPicks.bestArt(listOf(art(1, 3), art(2, 9), art(3, 5)))?.id)
    }

    @Test
    fun theLargerWinsAScoreTie() {
        assertEquals(2, ArtPicks.bestArt(listOf(art(1, 5, 342, 482), art(2, 5, 660, 930)))?.id)
    }

    @Test
    fun noArtIsNoMatch() {
        assertNull(ArtPicks.bestArt(emptyList()))
    }
}

/** Finding covers for the games that have none. */
class CoverFinderTest {
    private val core = FakeArtCore()
    private val finder = CoverFinder(core)

    private fun game(id: String, name: String, cover: String = "") = GameView(id = id, name = name, coverPath = cover)

    private fun known(name: String, gridId: Int, url: String) {
        core.games += (name to listOf(GridGame(id = gridId, name = name)))
        core.covers += (gridId to listOf(GridArt(id = gridId, url = url, score = 5)))
    }

    @Test
    fun aGameGetsTheBestCoverOfTheBestMatch() = runBlocking {
        core.games += ("Zelda" to listOf(GridGame(id = 1, name = "Zelda 2"), GridGame(id = 2, name = "Zelda")))
        core.covers += (2 to listOf(GridArt(id = 10, url = "low", score = 1), GridArt(id = 11, url = "high", score = 8)))
        assertTrue(finder.findCover(game("g", "Zelda")))
        assertEquals(listOf("search Zelda", "art cover|2", "setCover g|high"), core.calls)
    }

    @Test
    fun aGameSteamGridDBDoesNotKnowHasNoCover() = runBlocking {
        assertFalse(finder.findCover(game("g", "Homebrew")))
        assertEquals("nothing was set", listOf("search Homebrew"), core.calls)
    }

    @Test
    fun aKnownGameWithNoCoversHasNone() = runBlocking {
        core.games += ("Obscure" to listOf(GridGame(id = 5, name = "Obscure")))
        assertFalse(finder.findCover(game("g", "Obscure")))
        assertTrue(core.calls.none { it.startsWith("setCover") })
    }

    @Test
    fun onlyTheGamesWithoutACoverAreLookedFor() = runBlocking {
        known("A", 1, "ua")
        known("B", 2, "ub")
        val report = finder.findMissing(listOf(game("a", "A", cover = "/c/a.jpg"), game("b", "B")))
        assertEquals(FindReport.Done(found = 1, total = 1), report)
        assertEquals(listOf("search B", "art cover|2", "setCover b|ub"), core.calls)
    }

    @Test
    fun whenEveryGameHasACoverThereIsNothingToDo() = runBlocking {
        assertEquals(FindReport.NothingToDo, finder.findMissing(listOf(game("a", "A", cover = "/c"))))
        assertEquals(FindReport.NothingToDo, finder.findMissing(emptyList()))
        assertTrue(core.calls.isEmpty())
    }

    @Test
    fun theReportCountsTheCoversFoundOfTheGamesLookedFor() = runBlocking {
        known("A", 1, "ua")
        known("C", 3, "uc")
        val report = finder.findMissing(listOf(game("a", "A"), game("b", "Unknown"), game("c", "C")))
        assertEquals(FindReport.Done(found = 2, total = 3), report)
    }

    @Test
    fun aGameThatFailsAloneDoesNotStopTheOthers() = runBlocking {
        known("A", 1, "ua")
        known("C", 3, "uc")
        core.failTerms += "B"
        val report = finder.findMissing(listOf(game("a", "A"), game("b", "B"), game("c", "C")))
        assertEquals(FindReport.Done(found = 2, total = 3), report)
    }

    @Test
    fun twoFailuresInARowStopItWithWhatWasFound() = runBlocking {
        known("A", 1, "ua")
        core.failTerms += setOf("B", "C")
        val report = finder.findMissing(listOf(game("a", "A"), game("b", "B"), game("c", "C"), game("d", "D")))
        val failed = report as FindReport.Failed
        assertEquals(1, failed.found)
        assertEquals(4, failed.total)
        assertEquals("search C failed", failed.cause.message)
        assertTrue("D was never tried", core.calls.none { it == "search D" })
    }

    @Test
    fun aSuccessBetweenFailuresStartsTheCountAgain() = runBlocking {
        known("B", 2, "ub")
        known("D", 4, "ud")
        core.failTerms += setOf("A", "C")
        val report = finder.findMissing(listOf(game("a", "A"), game("b", "B"), game("c", "C"), game("d", "D")))
        assertEquals(FindReport.Done(found = 2, total = 4), report)
    }

    @Test
    fun theProgressShowsTheGameItIsOnAndEndsWhenDone() = runBlocking {
        core.gate = CompletableDeferred()
        known("A", 1, "ua")
        val running = async(Dispatchers.Unconfined) { finder.findMissing(listOf(game("a", "A"), game("b", "B"))) }
        assertEquals(FindProgress(done = 0, total = 2, name = "A", found = 0), finder.state.value)
        core.gate!!.complete(Unit)
        running.await()
        assertNull(finder.state.value)
    }

    @Test
    fun aSecondSearchWhileOneRunsIsIgnored() = runBlocking {
        core.gate = CompletableDeferred()
        val first = async(Dispatchers.Unconfined) { finder.findMissing(listOf(game("a", "A"))) }
        assertNull(finder.findMissing(listOf(game("b", "B"))))
        assertEquals(1, core.calls.count { it.startsWith("search") })
        core.gate!!.complete(Unit)
        first.await()
        Unit
    }
}
