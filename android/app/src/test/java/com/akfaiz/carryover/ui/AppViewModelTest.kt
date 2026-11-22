package com.akfaiz.carryover.ui

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.CoreEvent
import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.OpenSession
import com.akfaiz.carryover.core.PlanView
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.PromptKind
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.core.SessionStatus
import com.akfaiz.carryover.core.Settings
import com.akfaiz.carryover.core.SyncAction
import com.akfaiz.carryover.core.SyncAllOutcome
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.core.SyncOutcome
import com.akfaiz.carryover.core.SyncResult
import com.akfaiz.carryover.data.FindProgress
import com.akfaiz.carryover.session.PlayOutcome
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The library, syncing and playing: what the view model does with the core and the platform, and what it says. */
class AppViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private fun viewModel() = AppViewModel(h.services)

    private val eden = EmulatorView(id = "eden", name = "Eden", preset = "switch")
    private fun emulated(id: String, name: String) = GameView(
        id = id,
        name = name,
        emulators = listOf(ProfileView(emulatorId = "eden", name = "Eden", key = "$id.eden")),
    )

    // Loading

    @Test
    fun theLibraryLoadsAtStartWithOnlyWhatPlaysHere() {
        h.core.session.games = listOf(emulated("2", "zelda"), emulated("1", "Banjo"), GameView(id = "3", name = "PC game", slots = emptyList()))
        h.core.session.emulators = listOf(eden)
        val vm = viewModel()
        assertTrue(vm.state.value.loaded)
        assertEquals(listOf("Banjo", "zelda"), vm.state.value.games.map { it.name })
        assertEquals(listOf("Eden"), vm.state.value.emulators.map { it.name })
    }

    @Test
    fun cloudReadyFollowsTheSettings() {
        h.core.settingsCore.settings = Settings(remote = "gdrive")
        val vm = viewModel()
        assertTrue(vm.state.value.cloudReady)
    }

    @Test
    fun noCloudYetIsNotReady() {
        assertFalse(viewModel().state.value.cloudReady)
    }

    @Test
    fun aCoreThatFailedToStartLoadsNothing() {
        h.core.initError = "no core"
        val vm = viewModel()
        assertEquals("no core", vm.state.value.initError)
        assertFalse(vm.state.value.loaded)
        assertEquals(0, h.core.session.gamesCalls)
        assertEquals(emptyList<String>(), h.core.settingsCore.calls)
    }

    @Test
    fun aLibraryThatCantBeReadIsSaid() {
        h.core.session.gamesError = IllegalStateException("disk")
        val vm = viewModel()
        assertFalse(vm.state.value.loaded)
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_read_library), "disk")), h.said)
    }

    @Test
    fun settingsThatCantBeReadAreSaidAndTheLibraryStillLoads() {
        h.core.settingsCore.settingsError = IllegalStateException("no settings")
        val vm = viewModel()
        assertTrue(vm.state.value.loaded)
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_read_settings), "no settings")), h.said)
    }

    @Test
    fun aBurstOfLibraryEventsReloadsOnce() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        repeat(3) { h.core.events.tryEmit(CoreEvent.Library("")) }
        h.advance(100)
        assertEquals("not yet", before, h.core.session.gamesCalls)
        h.advance(100)
        assertEquals("once", before + 1, h.core.session.gamesCalls)
        assertNotNull(vm)
    }

    @Test
    fun connectingACloudReloadsTheLibrary() = runBlocking {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        h.settings.useCloud("gdrive", "Carryover")
        h.advance(200)
        assertTrue(vm.state.value.cloudReady)
        assertEquals(before + 1, h.core.session.gamesCalls)
    }

    @Test
    fun endingSessionsFollowThePlatform() {
        val vm = viewModel()
        h.platform.ending.value = setOf("g")
        assertEquals(setOf("g"), vm.state.value.ending)
        h.platform.ending.value = emptySet()
        assertEquals(emptySet<String>(), vm.state.value.ending)
    }

    // Events

    @Test
    fun sessionStatusIsKeptByProfileKeyOrElseByGame() {
        val vm = viewModel()
        h.core.events.tryEmit(CoreEvent.Status(SessionStatus(gameId = "g", key = "g.eden", phase = "playing")))
        h.core.events.tryEmit(CoreEvent.Status(SessionStatus(gameId = "h", phase = "sync")))
        assertEquals("playing", vm.state.value.sessions["g.eden"]?.phase)
        assertEquals("sync", vm.state.value.sessions["h"]?.phase)
    }

    @Test
    fun questionsAreQueuedAndAnswered() {
        val vm = viewModel()
        h.core.events.tryEmit(CoreEvent.PromptOpened(Prompt(id = "p1", kind = PromptKind.Lock)))
        h.core.events.tryEmit(CoreEvent.PromptOpened(Prompt(id = "p2", kind = PromptKind.Conflict)))
        assertEquals(listOf("p1", "p2"), vm.state.value.prompts.map { it.id })
        vm.answer(vm.state.value.prompts.first(), "yes")
        assertEquals(listOf("p1" to "yes"), h.core.responses)
        assertEquals(listOf("p2"), vm.state.value.prompts.map { it.id })
        h.core.events.tryEmit(CoreEvent.PromptClosed("p2"))
        assertTrue(vm.state.value.prompts.isEmpty())
    }

    @Test
    fun questionsAlreadyWaitingAtStartAreShown() {
        h.core.prompts = listOf(Prompt(id = "old", kind = PromptKind.SyncError))
        assertEquals(listOf("old"), viewModel().state.value.prompts.map { it.id })
    }

    // Playing

    @Test
    fun playingStartsTheGameAndRemembersIt() {
        val vm = viewModel()
        vm.play(gameOf(cover = "/c.jpg"), profileOf("g"))
        assertEquals(listOf("g|true|Zelda"), h.platform.played)
        assertEquals(listOf(Triple("g", "Zelda", "/c.jpg")), h.platform.recents)
        assertTrue(h.said.isEmpty())
        assertTrue(vm.state.value.starting.isEmpty())
    }

    @Test
    fun anEmulatorProfileIsPlayedByItsEmulatorsName() {
        viewModel().play(gameOf(), profileOf("g.eden", emulator = "Eden"))
        assertEquals(listOf("g.eden|false|Zelda (Eden)"), h.platform.played)
    }

    @Test
    fun playingOfflineSaysSo() {
        h.platform.playResult = Result.success(PlayOutcome.Started(offline = true))
        viewModel().play(gameOf(), profileOf())
        assertEquals(listOf(h.text(R.string.msg_offline_playing, "Zelda")), h.said)
        assertEquals("still remembered", 1, h.platform.recents.size)
    }

    @Test
    fun aCancelledStartSaysTheGameWasNotStarted() {
        h.platform.playResult = Result.success(PlayOutcome.Cancelled)
        viewModel().play(gameOf(), profileOf())
        assertEquals(listOf(h.text(R.string.msg_not_started, "Zelda")), h.said)
        assertTrue(h.platform.recents.isEmpty())
    }

    @Test
    fun aFailedStartSaysWhy() {
        h.platform.playResult = Result.success(PlayOutcome.Failed("Zelda: no cloud"))
        viewModel().play(gameOf(), profileOf())
        assertEquals(listOf("Zelda: no cloud"), h.said)
        assertTrue(h.platform.recents.isEmpty())
    }

    @Test
    fun aGameThatCantBeStartedHereSaysWhy() {
        h.platform.playResult = Result.failure(IllegalStateException("not installed"))
        val vm = viewModel()
        vm.play(gameOf(), profileOf())
        assertEquals(listOf(h.text(R.string.msg_error, "Zelda", "not installed")), h.said)
        assertTrue("the start is over", vm.state.value.starting.isEmpty())
    }

    @Test
    fun aSecondTapWhileTheGameIsStartingIsIgnored() {
        h.platform.playGate = CompletableDeferred()
        val vm = viewModel()
        vm.play(gameOf(), profileOf("g"))
        vm.play(gameOf(), profileOf("g"))
        assertEquals(1, h.platform.played.size)
        assertEquals(setOf("g"), vm.state.value.starting)
        h.platform.playGate!!.complete(Unit)
        assertTrue(vm.state.value.starting.isEmpty())
    }

    @Test
    fun doneAndShortcutsGoToThePlatform() {
        val vm = viewModel()
        vm.donePlaying("g")
        assertEquals(listOf("g"), h.platform.done)

        vm.addShortcut(gameOf(cover = "/c.jpg"), profileOf("g.eden", emulator = "Eden"))
        assertEquals(listOf(Triple("g.eden", "Zelda (Eden)", "/c.jpg")), h.platform.pinned)
        assertTrue(h.said.isEmpty())

        h.platform.canPin = false
        vm.addShortcut(gameOf(), profileOf())
        assertEquals(listOf(h.text(R.string.msg_no_shortcuts)), h.said)
    }

    @Test
    fun theAppOfAProfileIsWhatSessionsSays() {
        val vm = viewModel()
        assertEquals(Triple("com.example.game", "", ""), vm.launchSetup(profileOf(app = "com.example.game")))
        assertNull(vm.launchSetup(profileOf()))
    }

    @Test
    fun comingBackChecksTheSavesAndWakesTheWatcherOnlyForOpenSessions() {
        val vm = viewModel()
        vm.onResume()
        assertTrue("checkLocalChanges" in h.core.calls)
        assertEquals(0, h.platform.resumed)
        h.core.session.open = listOf(OpenSession(key = "g", game = "g"))
        vm.onResume()
        assertEquals(1, h.platform.resumed)
    }

    @Test
    fun comingBackWithACoreThatFailedDoesNothing() {
        h.core.initError = "no core"
        viewModel().onResume()
        assertTrue(h.core.calls.isEmpty())
        assertEquals(0, h.platform.resumed)
    }

    // Syncing

    @Test
    fun aSyncSaysWhatItDid() {
        val vm = viewModel()
        h.core.sync.outcome = SyncOutcome(SyncAction.Push)
        vm.sync("g", "Zelda")
        h.core.sync.outcome = SyncOutcome(SyncAction.Pull)
        vm.sync("g", "Zelda")
        h.core.sync.outcome = SyncOutcome(SyncAction.None)
        vm.sync("g", "Zelda")
        h.core.sync.outcome = SyncOutcome(offline = true)
        vm.sync("g", "Zelda")
        h.core.sync.outcome = SyncOutcome(cancelled = true)
        vm.sync("g", "Zelda")
        assertEquals(
            listOf(
                h.text(R.string.msg_sync_pushed, "Zelda"),
                h.text(R.string.msg_sync_pulled, "Zelda"),
                h.text(R.string.msg_sync_none, "Zelda"),
                h.text(R.string.msg_sync_offline, "Zelda"),
                h.text(R.string.msg_sync_cancelled, "Zelda"),
            ),
            h.said,
        )
    }

    @Test
    fun aFailedSyncSaysWhyWithTheGamesName() {
        h.core.sync.syncError = IllegalStateException("denied")
        viewModel().sync("g", "Zelda")
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_sync, "Zelda"), "denied")), h.said)
    }

    @Test
    fun aConflictAsksTheUserAndTheirChoiceSyncsAgain() {
        val vm = viewModel()
        h.core.sync.outcome = SyncOutcome(SyncAction.Conflict)
        vm.sync("g", "Zelda")
        val c = vm.state.value.conflict
        assertEquals("g", c?.key)
        assertEquals("Zelda", c?.name)
        assertTrue("the dialog says it, not a snackbar", h.said.isEmpty())

        h.core.sync.outcome = SyncOutcome(SyncAction.Push)
        vm.resolveConflict(Resolution.Local)
        assertEquals(listOf("g" to null, "g" to Resolution.Local), h.core.sync.synced)
        assertNull(vm.state.value.conflict)
        assertEquals(listOf(h.text(R.string.msg_sync_pushed, "Zelda")), h.said)
    }

    @Test
    fun decidingLaterLeavesTheConflictUnsynced() {
        val vm = viewModel()
        h.core.sync.outcome = SyncOutcome(SyncAction.Conflict)
        vm.sync("g", "Zelda")
        vm.resolveConflict(null)
        assertNull(vm.state.value.conflict)
        assertEquals(1, h.core.sync.synced.size)
    }

    @Test
    fun resolvingWithNoConflictDoesNothing() {
        val vm = viewModel()
        vm.resolveConflict(Resolution.Cloud)
        assertTrue(h.core.sync.synced.isEmpty())
    }

    @Test
    fun progressOfARunningSyncReachesTheState() {
        h.core.sync.gate = CompletableDeferred()
        val vm = viewModel()
        vm.sync("g", "Zelda")
        assertEquals(setOf("g"), vm.state.value.syncing.keys)
        h.core.events.tryEmit(CoreEvent.Sync(SyncEvent(gameId = "g", phase = "upload", bytes = 4, totalBytes = 9)))
        assertEquals(4L, vm.state.value.syncing["g"]?.bytes)
        vm.cancelSync("g")
        assertEquals(listOf("g"), h.core.sync.cancelled)
        h.core.sync.gate!!.complete(Unit)
        assertTrue(vm.state.value.syncing.isEmpty())
    }

    @Test
    fun syncAllSaysHowManyAndReloadsTheLibrary() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        h.core.sync.outcomeAll = SyncAllOutcome(listOf(SyncResult("a", "A", SyncAction.Push), SyncResult("b", "B", SyncAction.None)))
        vm.syncAll()
        assertEquals(listOf(h.plural(R.plurals.msg_syncall_done, 2, 2)), h.said)
        assertEquals(before + 1, h.core.session.gamesCalls)
        assertNull(vm.state.value.syncAll)
    }

    @Test
    fun syncAllNamesTheFirstFailureAndCountsConflicts() {
        val vm = viewModel()
        h.core.sync.outcomeAll = SyncAllOutcome(
            listOf(SyncResult("a", "A", SyncAction.Conflict), SyncResult("b", "B", error = "denied"), SyncResult("c", "C", error = "gone")),
        )
        vm.syncAll()
        assertEquals(listOf(h.plural(R.plurals.msg_syncall_failed, 2, 2, "B", "denied")), h.said)

        h.said.clear()
        h.core.sync.outcomeAll = SyncAllOutcome(listOf(SyncResult("a", "A", SyncAction.Conflict), SyncResult("c", "C", SyncAction.Conflict)))
        vm.syncAll()
        assertEquals(listOf(h.plural(R.plurals.msg_syncall_conflicts, 2, 2)), h.said)
    }

    @Test
    fun syncAllOfflineCancelledOrFailed() {
        val vm = viewModel()
        h.core.sync.outcomeAll = SyncAllOutcome(offline = true)
        vm.syncAll()
        h.core.sync.outcomeAll = SyncAllOutcome(cancelled = true)
        vm.syncAll()
        h.core.sync.syncError = IllegalStateException("net")
        vm.syncAll()
        assertEquals(
            listOf(
                h.text(R.string.msg_syncall_offline),
                h.text(R.string.msg_syncall_cancelled),
                h.text(R.string.msg_error, h.text(R.string.err_sync_all), "net"),
            ),
            h.said,
        )
    }

    @Test
    fun theTransferOfTheGameSyncAllIsOnIsShown() {
        h.core.sync.gate = CompletableDeferred()
        val vm = viewModel()
        vm.syncAll()
        h.core.events.tryEmit(CoreEvent.SyncAll(com.akfaiz.carryover.core.SyncAllProgress(done = 0, total = 2, name = "Zelda", key = "z")))
        h.core.events.tryEmit(CoreEvent.Sync(SyncEvent(gameId = "z", bytes = 7)))
        assertEquals("Zelda", vm.state.value.syncAll?.name)
        assertEquals(7L, vm.state.value.syncAllEvent?.bytes)
        h.core.sync.gate!!.complete(Unit)
        assertNull(vm.state.value.syncAll)
    }

    // Editing the library

    private fun error(what: Int, why: String) = h.text(R.string.msg_error, h.text(what), why)

    @Test
    fun theGameFileIsSavedOrCleared() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        vm.setRom("g.eden", "content://rom")
        vm.setRom("g.eden", "  ")
        assertEquals(listOf("setRom g.eden|content://rom", "setRom g.eden|"), h.core.calls.filter { it.startsWith("setRom") })
        assertEquals(listOf(h.text(R.string.msg_game_file_saved), h.text(R.string.msg_game_file_cleared)), h.said)
        assertEquals(before + 2, h.core.session.gamesCalls)
    }

    @Test
    fun aFailedEditSaysWhyAndDoesNotReload() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        h.core.failing += "setRom"
        vm.setRom("g", "x")
        assertEquals(listOf(error(R.string.err_save_game_file, "setRom failed")), h.said)
        assertEquals(before, h.core.session.gamesCalls)
    }

    @Test
    fun savePathsAndTheNativeApp() {
        val vm = viewModel()
        vm.setPaths("g", mapOf("saves" to "/sdcard/s"))
        vm.setNativeApp("g", "com.example.game")
        assertTrue("setPaths g|saves=/sdcard/s" in h.core.calls)
        assertTrue("setNativeApp g|com.example.game" in h.core.calls)
        assertEquals(listOf(h.text(R.string.msg_paths_saved), h.text(R.string.msg_app_saved)), h.said)
    }

    @Test
    fun anEmulatorSetupIsSavedWithItsPresetAndTrimmed() {
        val vm = viewModel()
        var done = 0
        vm.saveEmulator("", " RetroArch ", " com.retroarch ", " -e ROM {rom} ", " /data ", done = { done++ })
        val saved = h.core.savedEmulators.single()
        assertEquals("RetroArch", saved.name)
        assertEquals("com.retroarch", saved.exe)
        assertEquals("-e ROM {rom}", saved.args)
        assertEquals("/data", saved.dataDir)
        assertEquals("the preset of its app", "retroarch", saved.preset)
        assertEquals(1, done)
        assertEquals(listOf(h.text(R.string.msg_emulator_saved, " RetroArch ")), h.said)
    }

    @Test
    fun anEmulatorKeepsItsOwnPresetWhenEdited() {
        h.core.session.emulators = listOf(EmulatorView(id = "x", name = "Mine", preset = "switch"))
        val vm = viewModel()
        vm.saveEmulator("x", "Mine", "com.retroarch", "", "")
        assertEquals("switch", h.core.savedEmulators.single().preset)
    }

    @Test
    fun aFailedEmulatorSaveDoesNotCallDone() {
        h.core.failing += "saveEmulator"
        var done = 0
        viewModel().saveEmulator("", "X", "com.example", "", "", done = { done++ })
        assertEquals(0, done)
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_save_emulator, "X"), "saveEmulator failed")), h.said)
    }

    @Test
    fun anEmulatorCanBeRemovedFromThisDevice() {
        viewModel().unsetEmulator("eden", "Eden")
        assertTrue("unsetEmulator eden" in h.core.calls)
        assertEquals(listOf(h.text(R.string.msg_emulator_removed, "Eden")), h.said)
    }

    @Test
    fun aNewGameIsAddedTrimmedAndOpened() {
        var opened = ""
        viewModel().addGame(" Zelda ", " /sdcard/s ", done = { opened = it })
        assertTrue("addGame Zelda|/sdcard/s" in h.core.calls)
        assertEquals("new-id", opened)
    }

    @Test
    fun aNewEmulatedGameIsAddedAndOpened() {
        var opened = ""
        viewModel().addEmulatorGame(" Zelda ", "eden", " /rom ", " /saves ", done = { opened = it })
        assertTrue("addEmulatorGame Zelda|eden|/rom|/saves" in h.core.calls)
        assertEquals("emu-id", opened)
    }

    @Test
    fun aNewEmulatedGameWithoutAFolderLooksForItsSaves() {
        h.core.suggest.applied = 1
        var opened = ""
        viewModel().addEmulatorGame("Zelda", "eden", "/rom", "", done = { opened = it })
        assertEquals("the game opens without waiting for the lookup", "emu-id", opened)
        assertEquals(listOf("autofill emu-id.eden|"), h.core.suggest.calls)
        assertEquals(listOf(h.text(R.string.msg_suggest_autofilled)), h.said)
    }

    @Test
    fun aFolderTheUserChoseIsNotLookedUpAgain() {
        viewModel().addEmulatorGame("Zelda", "eden", "/rom", "/saves", done = {})
        assertTrue(h.core.suggest.calls.isEmpty())
    }

    @Test
    fun nothingFoundOrALookupThatFailsSaysNothing() {
        h.core.suggest.applied = 0
        viewModel().addEmulatorGame("Zelda", "eden", "/rom", "", done = {})
        h.core.suggest.failing += "autofill"
        viewModel().addEmulatorGame("Mario", "eden", "/rom", "", done = {})
        assertEquals(2, h.core.suggest.calls.size)
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun anAndroidGameIsAddedWithItsApp() {
        var opened = ""
        viewModel().addAndroidGame("Stardew", "com.chucklefish.stardewvalley", "/saves", done = { opened = it })
        assertEquals(listOf("addGame Stardew|/saves", "setNativeApp new-id|com.chucklefish.stardewvalley"), h.core.calls.filter { it.startsWith("addGame") || it.startsWith("setNativeApp") })
        assertEquals("new-id", opened)
    }

    @Test
    fun aFailedAddDoesNotOpenTheGame() {
        h.core.failing += "addGame"
        var opened = false
        viewModel().addGame("X", "/p", done = { opened = true })
        assertFalse(opened)
        assertEquals(listOf(error(R.string.err_add_game, "addGame failed")), h.said)
    }

    @Test
    fun renamingAndTheEmulatorsOfAGame() {
        val vm = viewModel()
        vm.renameGame("g", " New ")
        vm.addGameEmulator("g", "eden", "other", "Eden")
        vm.removeGameEmulator("g", "eden", "Eden")
        assertTrue("renameGame g|New" in h.core.calls)
        assertTrue("addGameEmulator g|eden|other" in h.core.calls)
        assertTrue("removeGameEmulator g|eden" in h.core.calls)
        assertEquals(
            listOf(h.text(R.string.msg_game_renamed), h.text(R.string.msg_game_emulator_added, "Eden"), h.text(R.string.msg_game_emulator_removed, "Eden")),
            h.said,
        )
    }

    @Test
    fun aFailedGameEditSaysWhy() {
        h.core.failing += "renameGame"
        viewModel().renameGame("g", "N")
        assertEquals(listOf(error(R.string.err_edit_game, "renameGame failed")), h.said)
    }

    @Test
    fun theWholeGameAndTheRomNameAreAskedOfTheCore() = runBlocking {
        val vm = viewModel()
        h.core.session.games = listOf(emulated("g", "Zelda"))
        assertEquals("Zelda", vm.fullGame("g")?.name)
        h.core.failing += "game"
        assertNull("a game that can't be read is none", vm.fullGame("g"))
        assertEquals("Guessed", vm.guessRomName("eden", "zelda.nsp"))
        h.core.failing += "guessRomName"
        assertEquals("", vm.guessRomName("eden", "zelda.nsp"))
    }

    @Test
    fun whatASyncWouldDoIsNullWhenTheCloudCantTell() = runBlocking {
        val vm = viewModel()
        h.core.sync.plan = PlanView(SyncAction.Push, hasRemote = true)
        assertEquals(SyncAction.Push, vm.plan("g")?.action)
        h.core.sync.planError = IllegalStateException("offline")
        assertNull(vm.plan("g"))
    }

    // Covers

    @Test
    fun aPickedImageBecomesTheCoverAndTheTempFileGoes() {
        val file = File.createTempFile("cover", ".jpg")
        h.platform.cover = file
        viewModel().setCover("g", "content://pick/1")
        assertEquals(listOf("setCover g|${file.name}"), h.core.calls.filter { it.startsWith("setCover") })
        assertFalse("the temp file is deleted", file.exists())
        assertEquals(listOf(h.text(R.string.msg_cover_changed)), h.said)
    }

    @Test
    fun aCoverTheCoreRefusesStillLeavesNoTempFile() {
        val file = File.createTempFile("cover", ".jpg")
        h.platform.cover = file
        h.core.failing += "setCover"
        viewModel().setCover("g", "content://pick/1")
        assertFalse(file.exists())
        assertEquals(listOf(error(R.string.err_cover, "setCover failed")), h.said)
    }

    @Test
    fun anImageThatCantBeReadIsSaid() {
        h.platform.coverError = IllegalStateException("unreadable image")
        viewModel().setCover("g", "content://pick/1")
        assertEquals(listOf(error(R.string.err_cover, "unreadable image")), h.said)
        assertTrue(h.core.calls.none { it.startsWith("setCover") })
    }

    // Deleting (to the trash)

    @Test
    fun aGameIsMovedToTheTrashAndTheLibraryReloaded() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        var done = 0
        vm.deleteGame("g", "Zelda", done = { done++ })
        assertTrue("deleteGame g" in h.core.trash.calls)
        assertEquals(listOf(h.text(R.string.msg_game_trashed, "Zelda")), h.said)
        assertEquals(before + 1, h.core.session.gamesCalls)
        assertEquals(1, done)
    }

    @Test
    fun aGameThatCantBeDeletedSaysWhyAndStaysOpen() {
        h.core.trash.failing += "deleteGame"
        val vm = viewModel()
        var done = 0
        vm.deleteGame("g", "Zelda", done = { done++ })
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_delete_game, "Zelda"), "deleteGame failed")), h.said)
        assertEquals("the page is not closed", 0, done)
    }

    @Test
    fun anEmulatorIsMovedToTheTrash() {
        var done = 0
        viewModel().deleteEmulator("eden", "Eden", done = { done++ })
        assertTrue("deleteEmulator eden" in h.core.trash.calls)
        assertEquals(listOf(h.text(R.string.msg_emulator_trashed, "Eden")), h.said)
        assertEquals(1, done)
    }

    @Test
    fun anEmulatorThatCantBeDeletedSaysWhy() {
        h.core.trash.failing += "deleteEmulator"
        var done = 0
        viewModel().deleteEmulator("eden", "Eden", done = { done++ })
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_delete_game, "Eden"), "deleteEmulator failed")), h.said)
        assertEquals(0, done)
    }

    // Covers from SteamGridDB

    private fun gameWithoutCover(id: String, name: String) = com.akfaiz.carryover.core.GameView(
        id = id,
        name = name,
        slots = listOf(com.akfaiz.carryover.core.SaveSlot("saves")),
        device = DeviceConfig(exe = "com.example.$id"),
    )

    private fun knownOnGrid(name: String, gridId: Int) {
        h.core.art.games += (name to listOf(com.akfaiz.carryover.core.GridGame(id = gridId, name = name)))
        h.core.art.covers += (gridId to listOf(com.akfaiz.carryover.core.GridArt(id = gridId, url = "u$gridId", score = 5)))
    }

    @Test
    fun theKeyShowsInTheState() {
        assertFalse(viewModel().state.value.hasGridKey)
        h.core.settingsCore.settings = com.akfaiz.carryover.core.Settings(steamGridDbKey = "key")
        assertTrue(viewModel().state.value.hasGridKey)
    }

    @Test
    fun coversAreFoundForTheGamesWithoutOneAndTheLibraryReloaded() {
        h.core.session.games = listOf(gameWithoutCover("a", "Alpha"), gameWithoutCover("b", "Beta"))
        knownOnGrid("Alpha", 1)
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        vm.findCovers()
        assertEquals(listOf("search Alpha", "art cover|1", "setCover a|u1", "search Beta"), h.core.art.calls)
        assertEquals(listOf(h.text(R.string.msg_covers_found, 1, 2)), h.said)
        assertEquals(before + 1, h.core.session.gamesCalls)
        assertNull(vm.state.value.findingCovers)
    }

    @Test
    fun withNoGameWithoutACoverTheUserIsToldSo() {
        h.core.session.games = emptyList()
        viewModel().findCovers()
        assertEquals(listOf(h.text(R.string.msg_covers_none)), h.said)
        assertTrue(h.core.art.calls.isEmpty())
    }

    @Test
    fun theSearchForCoversShowsItsProgress() {
        h.core.session.games = listOf(gameWithoutCover("a", "Alpha"))
        h.core.art.gate = CompletableDeferred()
        val vm = viewModel()
        vm.findCovers()
        assertEquals(FindProgress(done = 0, total = 1, name = "Alpha", found = 0), vm.state.value.findingCovers)
        h.core.art.gate!!.complete(Unit)
        assertNull(vm.state.value.findingCovers)
    }

    @Test
    fun aSearchThatFailsTwiceInARowSaysWhatWasFoundAndWhy() {
        h.core.session.games = listOf(gameWithoutCover("a", "Alpha"), gameWithoutCover("b", "Beta"), gameWithoutCover("c", "Gamma"))
        knownOnGrid("Alpha", 1)
        h.core.art.failTerms += setOf("Beta", "Gamma")
        viewModel().findCovers()
        assertEquals(
            listOf(h.text(R.string.msg_covers_found, 1, 3), h.text(R.string.msg_error, h.text(R.string.err_find_covers), "search Gamma failed")),
            h.said,
        )
    }

    @Test
    fun aBackgroundIsRemovedAndTheLibraryReloaded() {
        val vm = viewModel()
        val before = h.core.session.gamesCalls
        vm.clearBackground("g")
        assertTrue("clearBackground g" in h.core.art.calls)
        assertEquals(listOf(h.text(R.string.msg_background_removed)), h.said)
        assertEquals(before + 1, h.core.session.gamesCalls)
    }

    @Test
    fun aBackgroundThatCantBeRemovedSaysWhy() {
        h.core.art.failing += "clearBackground"
        viewModel().clearBackground("g")
        assertEquals(listOf(h.text(R.string.msg_error, h.text(R.string.err_remove_background), "clearBackground failed")), h.said)
    }

    @Test
    fun copiedSaysSo() {
        viewModel().copied()
        assertEquals(listOf(h.text(R.string.msg_copied)), h.said)
    }

    @Test
    fun theLibraryEntryOfAnUnknownDeviceDoesNotBreakAnything() {
        // A game with no profile at all still loads (it is left out as not playable here).
        h.core.session.games = listOf(GameView(id = "x", name = "Nothing", device = DeviceConfig()))
        val vm = viewModel()
        assertTrue(vm.state.value.loaded)
        assertTrue(vm.state.value.games.isEmpty())
    }
}
