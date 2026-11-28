package com.akfaiz.carryover.ui.library

import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.ui.gameOf
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** What the library lists, decided without Compose. */
class LibraryLogicTest {
    /** A game that is ready to play here: its save folder resolves and it has an app. */
    private fun ready(id: String, name: String = id, status: GameStatus = GameStatus()): GameView =
        gameOf(id, name).copy(resolvedPaths = mapOf("saves" to "/x"), status = status)

    /** A game whose save folder isn't set up here. */
    private fun notSetUp(id: String, name: String = id): GameView = gameOf(id, name)

    private fun ids(list: LibraryList) = list.shown.map { it.id }

    private val zelda = ready("zelda", "The Legend of Zelda")
    private val metroid = ready("metroid", "Metroid", GameStatus(conflict = true))
    private val mario = notSetUp("mario", "Super Mario")
    private val games = listOf(zelda, metroid, mario)

    @Test
    fun allListsEveryGameInTheLibrarysOrder() {
        assertEquals(listOf("zelda", "metroid", "mario"), ids(libraryList(games, emptyList(), "", LibraryFilter.All)))
    }

    @Test
    fun readyAndAttentionListTheirGames() {
        assertEquals(listOf("zelda", "metroid"), ids(libraryList(games, emptyList(), "", LibraryFilter.Ready)))
        assertEquals(listOf("metroid", "mario"), ids(libraryList(games, emptyList(), "", LibraryFilter.Attention)))
    }

    @Test
    fun aSearchMatchesPartOfTheNameWhateverItsCaseAndOuterSpaces() {
        assertEquals(listOf("zelda"), ids(libraryList(games, emptyList(), "  zELdA ", LibraryFilter.All)))
        assertEquals("a blank search matches everything", 3, libraryList(games, emptyList(), "   ", LibraryFilter.All).shown.size)
        assertEquals(emptyList<String>(), ids(libraryList(games, emptyList(), "pokemon", LibraryFilter.All)))
    }

    @Test
    fun aSearchAndAFilterBothApply() {
        assertEquals(listOf("mario"), ids(libraryList(games, emptyList(), "mario", LibraryFilter.Attention)))
        assertEquals(emptyList<String>(), ids(libraryList(games, emptyList(), "mario", LibraryFilter.Ready)))
    }

    @Test
    fun theCountsAreOfTheWholeLibraryWhateverIsShown() {
        val counts = mapOf(LibraryFilter.All to 3, LibraryFilter.Ready to 2, LibraryFilter.Attention to 2)
        assertEquals(counts, libraryList(games, emptyList(), "", LibraryFilter.All).counts)
        assertEquals(counts, libraryList(games, emptyList(), "zelda", LibraryFilter.Attention).counts)
    }

    @Test
    fun theFiltersShowOnlyWhenThereIsSomethingToFilterFor() {
        assertFalse(libraryList(listOf(zelda), emptyList(), "", LibraryFilter.All).showFilters)
        assertFalse("no games", libraryList(emptyList(), emptyList(), "", LibraryFilter.All).showFilters)
        assertTrue("a game needs attention", libraryList(listOf(zelda, metroid), emptyList(), "", LibraryFilter.All).showFilters)
        assertTrue("a filter that is on can be turned off", libraryList(listOf(zelda), emptyList(), "", LibraryFilter.Attention).showFilters)
    }

    @Test
    fun aGameBeingPlayedShowsTheFiltersOnlyWhenItIsNotReady() {
        // It never needs attention while played, but it still may not be ready.
        val played = notSetUp("g").copy(status = GameStatus(running = true))
        val list = libraryList(listOf(played), emptyList(), "", LibraryFilter.All)
        assertEquals(0, list.counts[LibraryFilter.Attention])
        assertTrue(list.showFilters)
        assertFalse(libraryList(listOf(ready("g", status = GameStatus(running = true))), emptyList(), "", LibraryFilter.All).showFilters)
    }

    @Test
    fun anEmulatorGameIsReadyOnceItsEmulatorHasAnApp() {
        val game = GameView(
            id = "g",
            name = "Zelda",
            emulators = listOf(
                ProfileView(emulatorId = "dolphin", name = "Dolphin", key = "g.dolphin", slots = listOf(SaveSlot("s")), resolvedPaths = mapOf("s" to "/x")),
            ),
        )
        val dolphin = EmulatorView(id = "dolphin", name = "Dolphin")
        val installed = dolphin.copy(device = EmulatorDevice(exe = "org.dolphinemu.dolphinemu"))
        assertEquals(emptyList<String>(), ids(libraryList(listOf(game), listOf(dolphin), "", LibraryFilter.Ready)))
        assertEquals(listOf("g"), ids(libraryList(listOf(game), listOf(installed), "", LibraryFilter.Ready)))
    }
}
