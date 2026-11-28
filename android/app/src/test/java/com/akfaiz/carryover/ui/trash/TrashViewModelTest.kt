package com.akfaiz.carryover.ui.trash

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Leftover
import com.akfaiz.carryover.core.TrashEmulator
import com.akfaiz.carryover.core.TrashGame
import com.akfaiz.carryover.core.TrashView
import com.akfaiz.carryover.ui.Harness
import kotlinx.coroutines.CompletableDeferred
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The trash page: restoring, deleting for good, and the cloud save folders no game uses. */
class TrashViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private val zelda = TrashGame(id = "g1", name = "Zelda", deletedAt = "2026-11-20T10:00:00Z", expiresAt = "2026-12-20T10:00:00Z", emulators = 1)
    private val mario = TrashGame(id = "g2", name = "Mario", deletedAt = "2026-11-21T10:00:00Z", expiresAt = "2026-12-21T10:00:00Z")
    private val eden = TrashEmulator(id = "e1", name = "Eden", deletedAt = "2026-11-20T10:00:00Z", expiresAt = "2026-12-20T10:00:00Z")

    private fun viewModel(): TrashViewModel {
        h.core.trash.view = TrashView(days = 30, games = listOf(zelda, mario), emulators = listOf(eden))
        return TrashViewModel(h.services)
    }

    private fun error(what: String, why: String) = h.text(R.string.msg_error, what, why)

    // Reading

    @Test
    fun theTrashIsReadAtStart() {
        val vm = viewModel()
        val s = vm.state.value
        assertTrue(s.loaded)
        assertEquals(30, s.days)
        assertEquals(listOf("Zelda", "Mario"), s.games.map { it.name })
        assertEquals(listOf("Eden"), s.emulators.map { it.name })
        assertNull("the cloud is looked at only when asked", s.leftovers)
        assertTrue("leftovers" !in h.core.trash.calls)
    }

    @Test
    fun anEmptyTrashIsLoadedAndEmpty() {
        val vm = TrashViewModel(h.services)
        assertTrue(vm.state.value.loaded)
        assertTrue(vm.state.value.isEmpty)
    }

    @Test
    fun aCoreThatFailedToStartReadsNothing() {
        h.core.initError = "no core"
        val vm = TrashViewModel(h.services)
        assertFalse(vm.state.value.loaded)
        assertTrue(h.core.trash.calls.isEmpty())
    }

    @Test
    fun aTrashThatCantBeReadIsSaid() {
        h.core.trash.failing += "trash"
        val vm = TrashViewModel(h.services)
        assertFalse(vm.state.value.loaded)
        assertEquals(listOf(error(h.text(R.string.err_read_trash), "trash failed")), h.said)
    }

    @Test
    fun readingAgainShowsWhatWasDeletedSince() {
        val vm = TrashViewModel(h.services)
        assertTrue(vm.state.value.isEmpty)
        // A game was deleted from its page while this view model lived on.
        h.core.trash.view = TrashView(days = 30, games = listOf(zelda))
        vm.refresh()
        assertEquals(listOf("Zelda"), vm.state.value.games.map { it.name })
    }

    // Restoring

    @Test
    fun aGameIsRestoredAndLeavesTheList() {
        val vm = viewModel()
        vm.restoreGame(zelda)
        assertTrue("restoreGame g1" in h.core.trash.calls)
        assertEquals(listOf("Mario"), vm.state.value.games.map { it.name })
        assertEquals(listOf(h.text(R.string.msg_restored, "Zelda")), h.said)
        assertTrue(vm.state.value.busy.isEmpty())
    }

    @Test
    fun anEmulatorIsRestored() {
        val vm = viewModel()
        vm.restoreEmulator(eden)
        assertTrue("restoreEmulator e1" in h.core.trash.calls)
        assertTrue(vm.state.value.emulators.isEmpty())
        assertEquals(listOf(h.text(R.string.msg_restored, "Eden")), h.said)
    }

    @Test
    fun aFailedRestoreIsSaidAndTheItemStays() {
        val vm = viewModel()
        h.core.trash.failing += "restoreEmulator"
        vm.restoreEmulator(eden)
        assertEquals(listOf(error(h.text(R.string.err_restore, "Eden"), "restoreEmulator failed")), h.said)
        assertEquals(listOf("Eden"), vm.state.value.emulators.map { it.name })
        assertTrue("not stuck busy", vm.state.value.busy.isEmpty())
    }

    // Deleting for good

    @Test
    fun aGameIsDeletedForGoodKeepingItsCloudSavesOrNot() {
        val vm = viewModel()
        vm.deleteGameForever(zelda, withSaves = false)
        vm.deleteGameForever(mario, withSaves = true)
        assertEquals(listOf("deleteGameForever g1|false", "deleteGameForever g2|true"), h.core.trash.calls.filter { it.startsWith("deleteGame") })
        assertTrue(vm.state.value.games.isEmpty())
        assertEquals(listOf(h.text(R.string.msg_deleted_forever, "Zelda"), h.text(R.string.msg_deleted_forever, "Mario")), h.said)
    }

    @Test
    fun anEmulatorIsDeletedForGood() {
        val vm = viewModel()
        vm.deleteEmulatorForever(eden, withSaves = true)
        assertTrue("deleteEmulatorForever e1|true" in h.core.trash.calls)
        assertTrue(vm.state.value.emulators.isEmpty())
        assertEquals(listOf(h.text(R.string.msg_deleted_forever, "Eden")), h.said)
    }

    @Test
    fun aDeleteThatFailsSaysWhyAndKeepsTheItem() {
        val vm = viewModel()
        h.core.trash.failing += "deleteGameForever"
        vm.deleteGameForever(zelda, withSaves = true)
        assertEquals(listOf(error(h.text(R.string.err_delete_game, "Zelda"), "deleteGameForever failed")), h.said)
        assertEquals(listOf("Zelda", "Mario"), vm.state.value.games.map { it.name })
        assertTrue(vm.state.value.busy.isEmpty())
    }

    @Test
    fun anItemWhoseActionIsRunningIsBusyAndNotStartedTwice() {
        val vm = viewModel()
        h.core.trash.gate = CompletableDeferred() // deleting the cloud saves takes a while
        vm.deleteGameForever(zelda, withSaves = true)
        assertEquals(setOf("g1"), vm.state.value.busy)
        vm.deleteGameForever(zelda, withSaves = true)
        vm.restoreGame(zelda)
        assertEquals("only the first reached the core", 1, h.core.trash.calls.count { it.startsWith("deleteGameForever") })
        assertTrue("restoreGame g1" !in h.core.trash.calls)
        vm.restoreGame(mario)
        assertTrue("another item is not blocked", "restoreGame g2" in h.core.trash.calls)

        h.core.trash.gate!!.complete(Unit)
        assertTrue(vm.state.value.busy.isEmpty())
    }

    // Cloud saves no game uses

    private val leftoverA = Leftover(key = "k1", gameId = "g1", gameName = "Zelda", files = 3, bytes = 2048)
    private val leftoverB = Leftover(key = "k2", gameId = "g2", gameName = "Mario", emulatorId = "e1", emulatorName = "Eden", files = 1, bytes = 10)

    @Test
    fun theCloudIsLookedAtOnlyWhenAsked() {
        h.core.trash.leftoverList = listOf(leftoverA, leftoverB)
        val vm = viewModel()
        assertNull(vm.state.value.leftovers)
        vm.lookInCloud()
        assertEquals(listOf("k1", "k2"), vm.state.value.leftovers?.map { it.key })
        assertFalse(vm.state.value.lookingInCloud)
    }

    @Test
    fun aLeftoverIsNamedWithItsEmulatorWhenThereIsOne() {
        assertEquals("Zelda", leftoverA.title)
        assertEquals("Mario (Eden)", leftoverB.title)
    }

    @Test
    fun noLeftoversIsAnEmptyListNotNull() {
        val vm = viewModel()
        vm.lookInCloud()
        assertEquals(emptyList<Leftover>(), vm.state.value.leftovers)
    }

    @Test
    fun aCloudThatCantBeReadIsSaidAndLeavesNothingLoaded() {
        h.core.trash.failing += "leftovers"
        val vm = viewModel()
        vm.lookInCloud()
        assertNull(vm.state.value.leftovers)
        assertFalse(vm.state.value.lookingInCloud)
        assertEquals(listOf(error(h.text(R.string.err_leftovers), "leftovers failed")), h.said)
    }

    @Test
    fun lookingTwiceAtOnceAsksTheCloudOnce() {
        h.core.trash.leftoversGate = CompletableDeferred()
        h.core.trash.leftoverList = listOf(leftoverA)
        val vm = viewModel()
        vm.lookInCloud()
        assertTrue(vm.state.value.lookingInCloud)
        vm.lookInCloud()
        assertEquals(1, h.core.trash.calls.count { it == "leftovers" })
        h.core.trash.leftoversGate!!.complete(Unit)
        assertFalse(vm.state.value.lookingInCloud)
        assertEquals(listOf("k1"), vm.state.value.leftovers?.map { it.key })
    }

    @Test
    fun aLeftoverIsBroughtBackAsAGame() {
        h.core.trash.leftoverList = listOf(leftoverA, leftoverB)
        val vm = viewModel()
        vm.lookInCloud()
        vm.restoreLeftover(leftoverB)
        assertTrue("restoreLeftover k2" in h.core.trash.calls)
        assertEquals(listOf("k1"), vm.state.value.leftovers?.map { it.key })
        assertEquals(listOf(h.text(R.string.msg_leftover_restored, "Mario (Eden)")), h.said)
    }

    @Test
    fun aLeftoverIsDeleted() {
        h.core.trash.leftoverList = listOf(leftoverA)
        val vm = viewModel()
        vm.lookInCloud()
        vm.deleteLeftover(leftoverA)
        assertTrue("deleteLeftover k1" in h.core.trash.calls)
        assertEquals(emptyList<Leftover>(), vm.state.value.leftovers)
        assertEquals(listOf(h.text(R.string.msg_leftover_deleted, "Zelda")), h.said)
    }

    @Test
    fun aFailedLeftoverActionIsSaid() {
        h.core.trash.leftoverList = listOf(leftoverA)
        val vm = viewModel()
        vm.lookInCloud()
        h.core.trash.failing += "deleteLeftover"
        vm.deleteLeftover(leftoverA)
        assertEquals(listOf(error(h.text(R.string.err_leftover_delete, "Zelda"), "deleteLeftover failed")), h.said)
        assertEquals(listOf("k1"), vm.state.value.leftovers?.map { it.key })
    }

    @Test
    fun deletingAGameForGoodKeepingItsSavesRefreshesTheLeftoversThatWereLooked() {
        h.core.trash.leftoverList = emptyList()
        val vm = viewModel()
        vm.lookInCloud()
        // The core now has a leftover: the saves of the game that was kept.
        h.core.trash.leftoverList = listOf(leftoverA)
        vm.deleteGameForever(zelda, withSaves = false)
        assertEquals(listOf("k1"), vm.state.value.leftovers?.map { it.key })
    }

    @Test
    fun deletingWithoutHavingLookedNeverCallsTheCloudForLeftovers() {
        val vm = viewModel()
        vm.deleteGameForever(zelda, withSaves = false)
        assertTrue("leftovers" !in h.core.trash.calls)
    }
}
