package com.akfaiz.carryover.ui.art

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame
import com.akfaiz.carryover.core.Settings
import com.akfaiz.carryover.ui.Harness
import kotlinx.coroutines.CompletableDeferred
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Finding a cover or a background for one game: search, choose the game, choose the image. */
class ArtViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private val zelda = GridGame(id = 7, name = "Zelda", year = 2017, verified = true)
    private val cover1 = GridArt(id = 1, url = "https://cdn2.steamgriddb.com/c1.png", thumb = "https://cdn2.steamgriddb.com/t1.png", score = 9)
    private val cover2 = GridArt(id = 2, url = "https://cdn2.steamgriddb.com/c2.png", thumb = "https://cdn2.steamgriddb.com/t2.png", score = 4)

    private fun viewModel(key: Boolean = true): ArtViewModel {
        if (key) {
            kotlinx.coroutines.runBlocking {
                h.core.settingsCore.settings = Settings(steamGridDbKey = "key")
                h.settings.refresh()
            }
        }
        h.core.art.games = mapOf("Zelda" to listOf(zelda))
        h.core.art.covers = mapOf(7 to listOf(cover1, cover2))
        h.core.art.backgrounds = mapOf(7 to listOf(cover2))
        return ArtViewModel(h.services)
    }

    private fun error(what: String, why: String) = h.text(R.string.msg_error, what, why)

    @Test
    fun openingSearchesForTheGamesName() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        val s = vm.state.value
        assertEquals("Zelda", s.query)
        assertEquals(listOf(zelda), s.games)
        assertFalse(s.searching)
        assertEquals(listOf("search Zelda"), h.core.art.calls)
    }

    @Test
    fun withoutAKeyNothingIsSearched() {
        val vm = viewModel(key = false)
        vm.open("g", "Zelda", ArtKind.Cover)
        assertFalse(vm.state.value.hasKey)
        assertNull(vm.state.value.games)
        assertTrue(h.core.art.calls.isEmpty())
    }

    @Test
    fun aGameWithNoNameIsNotSearchedButCanBe() {
        val vm = viewModel()
        vm.open("g", "  ", ArtKind.Cover)
        assertTrue(h.core.art.calls.isEmpty())
        vm.setQuery("Zelda")
        vm.search("Zelda")
        assertEquals(listOf(zelda), vm.state.value.games)
    }

    @Test
    fun searchingAgainReplacesTheGamesAndLeavesTheImages() {
        val vm = viewModel()
        h.core.art.games += ("Mario" to listOf(GridGame(id = 9, name = "Mario")))
        vm.open("g", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        assertEquals(zelda, vm.state.value.picked)
        vm.search("Mario")
        assertNull(vm.state.value.picked)
        assertNull(vm.state.value.grids)
        assertEquals(listOf("Mario"), vm.state.value.games?.map { it.name })
        assertEquals("Mario", vm.state.value.query)
    }

    @Test
    fun aBlankSearchDoesNothing() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        vm.search("   ")
        assertEquals(listOf("search Zelda"), h.core.art.calls)
    }

    @Test
    fun aSearchThatFailsIsSaidAndKeepsWhatWasShown() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        h.core.art.failing += "search"
        vm.search("Zelda again")
        assertEquals(listOf(error(h.text(R.string.err_find_art), "search failed")), h.said)
        assertEquals(listOf(zelda), vm.state.value.games)
        assertFalse(vm.state.value.searching)
    }

    @Test
    fun choosingAGameShowsItsCoversBestFirstAsTheyCame() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        assertEquals(listOf(cover1, cover2), vm.state.value.grids)
        assertFalse(vm.state.value.loadingArt)
        assertTrue("art cover|7" in h.core.art.calls)
    }

    @Test
    fun aBackgroundPageShowsBackgrounds() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Background)
        vm.pick(zelda)
        assertEquals(listOf(cover2), vm.state.value.grids)
        assertTrue("art hero|7" in h.core.art.calls)
    }

    @Test
    fun imagesThatCantBeListedAreSaidAndGoBackToTheGames() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        h.core.art.failing += "art"
        vm.pick(zelda)
        assertEquals(listOf(error(h.text(R.string.err_find_art), "art failed")), h.said)
        assertNull(vm.state.value.picked)
        assertEquals(listOf(zelda), vm.state.value.games)
    }

    @Test
    fun goingBackShowsTheGamesAgain() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        vm.backToGames()
        assertNull(vm.state.value.picked)
        assertNull(vm.state.value.grids)
        assertEquals(listOf(zelda), vm.state.value.games)
    }

    @Test
    fun anAnswerThatArrivesAfterTheUserWentBackIsDropped() {
        val vm = viewModel()
        vm.open("g", "Zelda", ArtKind.Cover)
        h.core.art.gate = CompletableDeferred()
        vm.pick(zelda)
        assertTrue(vm.state.value.loadingArt)
        vm.backToGames()
        h.core.art.gate!!.complete(Unit)
        assertNull("the late images are not shown", vm.state.value.grids)
        assertNull(vm.state.value.picked)
    }

    @Test
    fun aSlowOldSearchDoesNotOverwriteANewerOne() {
        val vm = viewModel()
        h.core.art.games += ("Mario" to listOf(GridGame(id = 9, name = "Mario")))
        h.core.art.gate = CompletableDeferred()
        vm.open("g", "Zelda", ArtKind.Cover) // waits
        h.core.art.gate = null
        vm.search("Mario") // answers at once
        assertEquals(listOf("Mario"), vm.state.value.games?.map { it.name })
        assertFalse(vm.state.value.searching)
    }

    @Test
    fun aChosenCoverIsDownloadedAsTheCoverAndThePageCloses() {
        val vm = viewModel()
        vm.open("g1", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        var closed = 0
        vm.apply(cover1) { closed++ }
        assertTrue("setCover g1|${cover1.url}" in h.core.art.calls)
        assertEquals(listOf(h.text(R.string.msg_cover_changed)), h.said)
        assertEquals(1, closed)
        assertFalse(vm.state.value.applying)
    }

    @Test
    fun aChosenBackgroundIsDownloadedAsTheBackground() {
        val vm = viewModel()
        vm.open("g1", "Zelda", ArtKind.Background)
        vm.pick(zelda)
        var closed = 0
        vm.apply(cover2) { closed++ }
        assertTrue("setBackground g1|${cover2.url}" in h.core.art.calls)
        assertTrue(h.core.art.calls.none { it.startsWith("setCover") })
        assertEquals(listOf(h.text(R.string.msg_background_changed)), h.said)
        assertEquals(1, closed)
    }

    @Test
    fun anImageThatCantBeDownloadedIsSaidAndThePageStaysOpen() {
        val vm = viewModel()
        vm.open("g1", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        h.core.art.failing += "setCover"
        var closed = 0
        vm.apply(cover1) { closed++ }
        assertEquals(listOf(error(h.text(R.string.err_apply_art), "setCover failed")), h.said)
        assertEquals(0, closed)
        assertFalse(vm.state.value.applying)
        assertEquals("the images are still there to choose again", listOf(cover1, cover2), vm.state.value.grids)
    }

    @Test
    fun aSecondTapWhileAnImageDownloadsIsIgnored() {
        val vm = viewModel()
        vm.open("g1", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        h.core.art.applyGate = CompletableDeferred()
        vm.apply(cover1) {}
        assertTrue(vm.state.value.applying)
        vm.apply(cover2) {}
        assertEquals(1, h.core.art.calls.count { it.startsWith("setCover") })
        h.core.art.applyGate!!.complete(Unit)
        assertFalse(vm.state.value.applying)
    }

    @Test
    fun openingAgainStartsAfresh() {
        val vm = viewModel()
        vm.open("g1", "Zelda", ArtKind.Cover)
        vm.pick(zelda)
        vm.open("g2", "Other", ArtKind.Background)
        val s = vm.state.value
        assertEquals("g2", s.gameId)
        assertEquals(ArtKind.Background, s.kind)
        assertNull(s.picked)
        assertNull(s.grids)
        assertEquals("Other", s.query)
    }
}
