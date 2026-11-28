package com.akfaiz.carryover.ui.nav

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** Which way pages move, and which keep what they saved. */
class PageMoveTest {
    @Test
    fun openingGoesForwardAndBackGoesBack() {
        val nav = Nav()
        val library = nav.place
        nav.go(Screen.Game("a"))
        val game = nav.place
        assertEquals(PageMove.Forward, pageMove(library, game))
        nav.go(Screen.EditGame("a"))
        assertEquals(PageMove.Forward, pageMove(game, nav.place))
        val editing = nav.place
        nav.back()
        assertEquals(PageMove.Back, pageMove(editing, nav.place))
    }

    @Test
    fun oneTabForAnotherOnlyFades() {
        val nav = Nav()
        val library = nav.place
        nav.go(Screen.Settings)
        assertEquals(PageMove.Fade, pageMove(library, nav.place))
    }

    @Test
    fun aTabOpenedFromAPageOverAnotherGoesBack() {
        val nav = Nav()
        nav.go(Screen.AddGame)
        val adding = nav.place
        nav.go(Screen.Emulators)
        assertEquals(PageMove.Back, pageMove(adding, nav.place))
    }

    @Test
    fun aPageKeepsItsStateUntilItIsClosed() {
        val nav = Nav()
        nav.go(Screen.Game("a"))
        val game = nav.place.stateKey
        nav.go(Screen.EditGame("a"))
        assertTrue(game in nav.openStateKeys)
        nav.back()
        nav.back()
        assertFalse(game in nav.openStateKeys)
    }

    @Test
    fun tabsAlwaysKeepTheirState() {
        val nav = Nav()
        val library = nav.place.stateKey
        nav.go(Screen.Settings)
        nav.go(Screen.Cloud)
        assertTrue(library in nav.openStateKeys)
    }

    @Test
    fun theSamePageAtTwoDepthsKeepsTwoStates() {
        assertNotEquals(Place(Screen.Game("a"), 1).stateKey, Place(Screen.Game("a"), 2).stateKey)
        assertNotEquals(Place(Screen.Game("a"), 1).stateKey, Place(Screen.Game("b"), 1).stateKey)
    }
}
