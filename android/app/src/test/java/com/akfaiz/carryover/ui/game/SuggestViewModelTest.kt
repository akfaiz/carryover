package com.akfaiz.carryover.ui.game

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SaveKind
import com.akfaiz.carryover.core.SaveSuggestion
import com.akfaiz.carryover.core.SaveSuggestions
import com.akfaiz.carryover.ui.Harness
import kotlinx.coroutines.CompletableDeferred
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The suggested save folders dialog of a game's emulator profile. */
class SuggestViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private val card = SaveSuggestion(game = "Luigi", path = ":saf:a/b/GC", include = listOf("*.gci"), exists = true)
    private val states = SaveSuggestion(game = "Luigi", path = ":saf:a/b/States", kind = SaveKind.None)

    private fun viewModel(): SuggestViewModel {
        h.core.suggest.found = SaveSuggestions(listOf(card, states), perGame = true)
        return SuggestViewModel(h.services)
    }

    @Test
    fun itStartsClosed() {
        val s = viewModel().state.value
        assertNull(s.key)
        assertFalse(s.loading)
    }

    @Test
    fun openingLooksTheLocationsUp() {
        val vm = viewModel()
        vm.open("g1.e1")
        val s = vm.state.value
        assertEquals("g1.e1", s.key)
        assertFalse(s.loading)
        assertEquals(listOf(card, states), s.result?.suggestions)
        assertNull(s.error)
        assertEquals(listOf("suggest g1.e1|"), h.core.suggest.calls)
    }

    @Test
    fun itIsLoadingUntilTheAnswerComes() {
        val vm = viewModel()
        h.core.suggest.gate = CompletableDeferred()
        vm.open("g1.e1")
        assertTrue(vm.state.value.loading)
        assertNull(vm.state.value.result)
        h.core.suggest.gate!!.complete(Unit)
        assertFalse(vm.state.value.loading)
        assertEquals(2, vm.state.value.result?.suggestions?.size)
    }

    @Test
    fun aLookupThatFailsShowsTheCoresWordsInTheDialogNotASnackbar() {
        val vm = viewModel()
        h.core.suggest.failing += "suggest"
        vm.open("g1.e1")
        val s = vm.state.value
        assertEquals("the core's words, as a sentence", "Suggest failed", s.error)
        assertNull(s.result)
        assertFalse(s.loading)
        assertEquals("the dialog is where it is shown", "g1.e1", s.key)
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun theAnswerOfADialogThatWasClosedIsDropped() {
        val vm = viewModel()
        h.core.suggest.gate = CompletableDeferred()
        vm.open("g1.e1")
        vm.close()
        h.core.suggest.gate!!.complete(Unit)
        assertNull(vm.state.value.key)
        assertNull(vm.state.value.result)
    }

    @Test
    fun theLateAnswerOfAnEarlierProfileIsDropped() {
        val vm = viewModel()
        val slow = CompletableDeferred<Unit>()
        h.core.suggest.gate = slow
        vm.open("g1.e1")
        h.core.suggest.gate = null
        h.core.suggest.found = SaveSuggestions(listOf(states))
        vm.open("g2.e1")
        assertEquals("g2.e1", vm.state.value.key)
        assertEquals(listOf(states), vm.state.value.result?.suggestions)
        slow.complete(Unit)
        assertEquals("g2.e1", vm.state.value.key)
        assertEquals(listOf(states), vm.state.value.result?.suggestions)
    }

    @Test
    fun usingASuggestionSetsItSaysSoAndCloses() {
        val vm = viewModel()
        vm.open("g1.e1")
        vm.use(card)
        assertEquals(listOf("suggest g1.e1|", "use g1.e1|:saf:a/b/GC"), h.core.suggest.calls)
        assertEquals(listOf(h.text(R.string.msg_suggest_used)), h.said)
        assertNull(vm.state.value.key)
        assertFalse(vm.state.value.applying)
    }

    @Test
    fun aSuggestionTheCoreRefusesIsSaidAndTheDialogStaysOpen() {
        val vm = viewModel()
        vm.open("g1.e1")
        h.core.suggest.failing += "use"
        vm.use(card)
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_suggest_use), "use failed")), h.said)
        assertEquals("g1.e1", vm.state.value.key)
        assertFalse(vm.state.value.applying)
    }

    @Test
    fun nothingIsUsedWhileClosed() {
        val vm = viewModel()
        vm.use(card)
        assertTrue(h.core.suggest.calls.isEmpty())
    }
}
