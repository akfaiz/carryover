package com.akfaiz.carryover.ui

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Backup
import com.akfaiz.carryover.core.BackupKind
import com.akfaiz.carryover.core.SyncOutcome
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The backups page of a profile: list, back up now, pin, delete, restore. */
class BackupsViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private val cloud = Backup(name = "c1", time = "2026-11-20T10:00:00Z", deviceName = "PC", kind = BackupKind.Cloud)
    private val manual = Backup(name = "m1", time = "2026-11-21T10:00:00Z", deviceName = "Pixel", kind = BackupKind.Manual, pinned = true)

    private fun viewModel(key: String = "g1"): BackupsViewModel {
        h.core.backup.backups = mapOf(key to listOf(manual, cloud))
        val vm = BackupsViewModel(h.services)
        vm.open(key)
        return vm
    }

    private fun error(what: Int, why: String) = h.text(R.string.msg_error, h.text(what), why)

    @Test
    fun openingListsTheBackupsOfTheProfile() {
        val vm = viewModel()
        val s = vm.state.value
        assertEquals("g1", s.key)
        assertTrue(s.loaded)
        assertFalse(s.loadFailed)
        assertEquals(listOf(manual, cloud), s.backups)
        assertEquals(listOf("backups g1"), h.core.backup.calls)
    }

    @Test
    fun anotherProfileStartsWithAnEmptyListAndDropsTheLateAnswerOfTheFirst() {
        val vm = viewModel("g1")
        h.core.backup.backups += "g2" to listOf(cloud)
        val slow = CompletableDeferred<Unit>()
        h.core.backup.listGate = slow
        vm.refresh() // g1's list is on its way, and slow
        h.core.backup.listGate = null
        vm.open("g2")
        assertEquals("g2", vm.state.value.key)
        assertEquals(listOf(cloud), vm.state.value.backups)
        slow.complete(Unit) // g1's answer arrives after g2's
        assertEquals(listOf(cloud), vm.state.value.backups)
        assertEquals("g2", vm.state.value.key)
    }

    @Test
    fun anotherProfileStartsEmptyUntilItsListArrives() {
        val vm = viewModel("g1")
        h.core.backup.backups += "g2" to listOf(cloud)
        h.core.backup.listGate = CompletableDeferred()
        vm.open("g2")
        assertTrue(vm.state.value.backups.isEmpty())
        assertFalse(vm.state.value.loaded)
        h.core.backup.listGate!!.complete(Unit)
        assertEquals(listOf(cloud), vm.state.value.backups)
        assertTrue(vm.state.value.loaded)
    }

    @Test
    fun openingTheSameProfileAgainReadsTheListAgain() {
        val vm = viewModel()
        h.core.backup.backups += "g1" to listOf(cloud)
        vm.open("g1")
        assertEquals(listOf(cloud), vm.state.value.backups)
        assertEquals(2, h.core.backup.calls.count { it == "backups g1" })
    }

    @Test
    fun aListThatCannotBeReadSaysSoAndCanBeTriedAgain() {
        h.core.backup.failing += "backups"
        val vm = viewModel()
        assertTrue(vm.state.value.loaded)
        assertTrue(vm.state.value.loadFailed)
        assertEquals(listOf(error(R.string.err_read_backups, "backups failed")), h.said)
        h.core.backup.failing.clear()
        vm.refresh()
        assertFalse(vm.state.value.loadFailed)
        assertEquals(listOf(manual, cloud), vm.state.value.backups)
    }

    @Test
    fun aFailedRefreshKeepsWhatWasShown() {
        val vm = viewModel()
        h.core.backup.failing += "backups"
        vm.refresh()
        assertEquals(listOf(manual, cloud), vm.state.value.backups)
    }

    @Test
    fun backingUpNowMakesAPinnedBackupAndListsIt() {
        val vm = viewModel()
        vm.backupNow()
        assertEquals(listOf(h.text(R.string.msg_backup_made)), h.said)
        assertFalse(vm.state.value.making)
        assertEquals(3, vm.state.value.backups.size)
        assertEquals(BackupKind.Manual, vm.state.value.backups.first().kind)
        assertTrue("backupNow g1" in h.core.backup.calls)
    }

    @Test
    fun onlyOneActionRunsAtATime() {
        val vm = viewModel()
        h.core.backup.gate = CompletableDeferred()
        vm.backupNow()
        assertTrue(vm.state.value.making)
        assertTrue(vm.state.value.busy)
        assertTrue(vm.state.value.cancellable)
        vm.backupNow()
        vm.restore(cloud)
        vm.delete(cloud)
        vm.togglePinned(cloud)
        assertEquals(1, h.core.backup.calls.count { it.startsWith("backupNow") })
        assertTrue(h.core.backup.calls.none { it.startsWith("restore") || it.startsWith("delete") || it.startsWith("pin") })
        h.core.backup.gate!!.complete(Unit)
        assertFalse(vm.state.value.busy)
    }

    @Test
    fun backingUpOfflineSaysOfflineAndCancellingSaysNothing() {
        val vm = viewModel()
        h.core.backup.outcome = SyncOutcome(offline = true)
        vm.backupNow()
        assertEquals(listOf(h.text(R.string.msg_backup_offline)), h.said)
        assertFalse(vm.state.value.making)
        h.said.clear()
        h.core.backup.outcome = SyncOutcome(cancelled = true)
        vm.backupNow()
        assertTrue(h.said.isEmpty())
        assertEquals(2, vm.state.value.backups.size)
    }

    @Test
    fun aFailedBackupSaysWhy() {
        val vm = viewModel()
        h.core.backup.failing += "backupNow"
        vm.backupNow()
        assertEquals(listOf(error(R.string.err_backup_now, "backupNow failed")), h.said)
        assertFalse(vm.state.value.making)
    }

    @Test
    fun pinningFlipsTheBackupAndListsAgain() {
        val vm = viewModel()
        vm.togglePinned(cloud)
        assertTrue("pin g1|c1|true" in h.core.backup.calls)
        assertTrue(vm.state.value.backups.first { it.name == "c1" }.pinned)
        vm.togglePinned(manual)
        assertTrue("pin g1|m1|false" in h.core.backup.calls)
        assertFalse(vm.state.value.backups.first { it.name == "m1" }.pinned)
        assertNull(vm.state.value.acting)
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun deletingRemovesTheBackup() {
        val vm = viewModel()
        vm.delete(cloud)
        assertEquals(listOf(h.text(R.string.msg_backup_deleted)), h.said)
        assertEquals(listOf(manual), vm.state.value.backups)
        assertNull(vm.state.value.acting)
    }

    @Test
    fun aFailedDeleteKeepsItAndSaysWhy() {
        val vm = viewModel()
        h.core.backup.failing += "delete"
        vm.delete(cloud)
        assertEquals(listOf(error(R.string.err_backup_delete, "delete failed")), h.said)
        assertEquals(2, vm.state.value.backups.size)
        assertNull(vm.state.value.acting)
    }

    @Test
    fun restoringRunsOnTheBackupAndCanBeCancelled() {
        val vm = viewModel()
        h.core.backup.gate = CompletableDeferred()
        vm.restore(cloud)
        assertEquals("c1", vm.state.value.acting)
        assertTrue(vm.state.value.restoring)
        assertTrue(vm.state.value.cancellable)
        vm.cancel()
        assertEquals(listOf("g1"), h.core.sync.cancelled)
        h.core.backup.gate!!.complete(Unit)
        assertEquals(listOf(h.text(R.string.msg_backup_restored)), h.said)
        assertNull(vm.state.value.acting)
        assertFalse(vm.state.value.restoring)
        assertTrue("restore g1|c1" in h.core.backup.calls)
    }

    @Test
    fun aCancelledRestoreIsNotReported() {
        val vm = viewModel()
        h.core.backup.outcome = SyncOutcome(cancelled = true)
        vm.restore(cloud)
        assertTrue(h.said.isEmpty())
        assertNull(vm.state.value.acting)
    }

    @Test
    fun aFailedRestoreSaysWhy() {
        val vm = viewModel()
        h.core.backup.failing += "restore"
        vm.restore(cloud)
        assertEquals(listOf(error(R.string.err_backup_restore, "restore failed")), h.said)
        assertNull(vm.state.value.acting)
        assertFalse(vm.state.value.restoring)
    }

    @Test
    fun onlyBackingUpAndRestoringCanBeCancelled() {
        assertFalse(BackupsState().cancellable)
        assertFalse(BackupsState(acting = "c1").cancellable)
        assertTrue(BackupsState(acting = "c1", restoring = true).cancellable)
        assertTrue(BackupsState(making = true).cancellable)
    }

    @Test
    fun theKeptCountFollowsTheSetting() = runBlocking {
        val vm = viewModel()
        assertEquals(0, vm.state.value.kept)
        h.settings.setBackupsKept(7)
        assertEquals(7, vm.state.value.kept)
        vm.open("g2")
        assertEquals("a new profile keeps the count", 7, vm.state.value.kept)
    }
}
