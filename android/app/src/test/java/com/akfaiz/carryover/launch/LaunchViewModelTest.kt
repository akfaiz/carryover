package com.akfaiz.carryover.launch

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.CoreEvent
import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorDevice
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.PromptKind
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.core.SessionStatus
import com.akfaiz.carryover.session.PlayOutcome
import com.akfaiz.carryover.ui.FakeTexts
import com.akfaiz.carryover.ui.Harness
import com.akfaiz.carryover.ui.components.appLabel
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

private class FakeLaunchPlatform(override val request: LaunchRequest) : LaunchPlatform {
    override val ownPackage = "com.akfaiz.carryover"
    val labels = mutableMapOf("org.ppsspp.ppsspp" to "PPSSPP app")
    var forwardOk = true
    val prepared = mutableListOf<String>()

    var playResult: Result<PlayOutcome> = Result.success(PlayOutcome.Started(false))
    var playForwardResult: PlayOutcome = PlayOutcome.Started(false)
    var trackResult: PlayOutcome = PlayOutcome.Started(false)
    val played = mutableListOf<String>()
    val playedForward = mutableListOf<Pair<String, String>>()
    val tracked = mutableListOf<Pair<String, String>>()
    var startError: String? = null
    var startedWithoutSync = 0
    val recents = mutableListOf<String>()
    val logs = mutableListOf<String>()

    override fun appLabel(pkg: String) = labels[pkg] ?: pkg

    override fun prepareForward(target: String): Boolean {
        prepared += target
        return forwardOk
    }

    override suspend fun play(cand: Forward.Candidate, emulators: List<EmulatorView>): Result<PlayOutcome> {
        played += cand.profile.key
        return playResult
    }

    override suspend fun playForward(cand: Forward.Candidate, pkg: String): PlayOutcome {
        playedForward += cand.profile.key to pkg
        return playForwardResult
    }

    override suspend fun track(cand: Forward.Candidate, pkg: String): PlayOutcome {
        tracked += cand.profile.key to pkg
        return trackResult
    }

    override fun startWithoutSync(): String? {
        startedWithoutSync++
        return startError
    }

    override fun rememberRecent(cand: Forward.Candidate) {
        recents += cand.profile.key
    }

    override fun log(what: String, e: Throwable) {
        logs += "$what: ${e.message}"
    }
}

/** A launch from a frontend or a shortcut: what it asks the platform and what the screen shows. */
class LaunchViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private val ppsspp = EmulatorView(id = "e1", name = "PPSSPP", device = EmulatorDevice(exe = "org.ppsspp.ppsspp/.PpssppActivity"))

    private fun psp(id: String, name: String, rom: String = "") = GameView(
        id = id,
        name = name,
        emulators = listOf(ProfileView(emulatorId = "e1", name = "PPSSPP", key = "$id.e1", rom = rom)),
    )

    private val native = GameView(
        id = "n",
        name = "Stardew",
        slots = listOf(SaveSlot("saves")),
        device = DeviceConfig(exe = "com.chucklefish.stardewvalley"),
    )

    init {
        h.core.session.games = listOf(psp("a", "Game A", rom = "/sdcard/ROMs/a.iso"), psp("b", "Game B"), native)
        h.core.session.emulators = listOf(ppsspp)
    }

    private fun request(
        key: String? = null,
        track: Boolean = false,
        target: String? = null,
        data: String? = null,
        extras: Map<String, String> = emptyMap(),
    ) = LaunchRequest(key, track, target, data, extras)

    private var platform = FakeLaunchPlatform(request())

    private fun launch(r: LaunchRequest): LaunchViewModel {
        platform = FakeLaunchPlatform(r)
        return LaunchViewModel(h.core, platform, FakeTexts)
    }

    private fun LaunchViewModel.error() = (state.value as LaunchState.Error).message

    // Start

    @Test
    fun itStartsWorking() {
        val vm = launch(request())
        assertEquals(LaunchState.Working(h.text(R.string.status_checking)), vm.state.value)
    }

    @Test
    fun aCoreThatFailedToStartIsTheError() {
        h.core.initError = "no core"
        val vm = launch(request(key = "a.e1"))
        vm.start()
        assertEquals(h.text(R.string.launch_init_failed, "no core"), vm.error())
        assertTrue(platform.played.isEmpty())
    }

    @Test
    fun aLaunchIsHandledOnce() {
        val vm = launch(request(key = "a.e1"))
        vm.start()
        vm.start()
        assertEquals(listOf("a.e1"), platform.played)
    }

    @Test
    fun aLaunchWithNothingInItIsAnError() {
        val vm = launch(request())
        vm.start()
        assertEquals(h.text(R.string.launch_nothing), vm.error())
    }

    @Test
    fun aShortcutOfAGameThatIsGoneIsAnError() {
        val vm = launch(request(key = "zzz"))
        vm.start()
        assertEquals(h.text(R.string.launch_game_gone), vm.error())
    }

    @Test
    fun anErrorInTheCoreIsShownAndLogged() {
        h.core.session.gamesError = IllegalStateException("db locked")
        val vm = launch(request(key = "a.e1"))
        vm.start()
        assertEquals("db locked", vm.error())
        assertEquals(listOf("launch: db locked"), platform.logs)
    }

    // A shortcut

    @Test
    fun aShortcutPlaysItsProfileAndIsRemembered() {
        val vm = launch(request(key = "a.e1"))
        vm.start()
        assertEquals(listOf("a.e1"), platform.played)
        assertEquals(listOf("a.e1"), platform.recents)
        assertEquals(LaunchState.Done, vm.state.value)
    }

    @Test
    fun aCancelledShortcutJustCloses() {
        val vm = launch(request(key = "a.e1"))
        platform.playResult = Result.success(PlayOutcome.Cancelled)
        vm.start()
        assertEquals(LaunchState.Done, vm.state.value)
        assertTrue(platform.recents.isEmpty())
    }

    @Test
    fun aShortcutThatFailsShowsWhy() {
        val vm = launch(request(key = "a.e1"))
        platform.playResult = Result.success(PlayOutcome.Failed("Game A: no cloud"))
        vm.start()
        assertEquals("Game A: no cloud", vm.error())
    }

    @Test
    fun aGameThatCantBeStartedHereShowsWhy() {
        val vm = launch(request(key = "a.e1"))
        platform.playResult = Result.failure(IllegalStateException("not installed"))
        vm.start()
        assertEquals(h.text(R.string.msg_error, "Game A (PPSSPP)", "not installed"), vm.error())
    }

    // Track

    @Test
    fun trackSyncsTheRunningGameWithTheEmulatorsPackage() {
        val vm = launch(request(key = "a.e1", track = true))
        vm.start()
        assertEquals(listOf("a.e1" to "org.ppsspp.ppsspp"), platform.tracked)
        assertTrue("nothing is started", platform.played.isEmpty())
        assertEquals(LaunchState.Done, vm.state.value)
    }

    @Test
    fun trackThatFailsShowsWhy() {
        val vm = launch(request(key = "a.e1", track = true))
        platform.trackResult = PlayOutcome.Failed("no cloud")
        vm.start()
        assertEquals("no cloud", vm.error())
    }

    @Test
    fun aCancelledTrackJustCloses() {
        val vm = launch(request(key = "a.e1", track = true))
        platform.trackResult = PlayOutcome.Cancelled
        vm.start()
        assertEquals(LaunchState.Done, vm.state.value)
    }

    // A frontend

    @Test
    fun aFrontendWithAKnownFilePlaysItsGame() {
        val vm = launch(request(target = "org.ppsspp.ppsspp/.PpssppActivity", data = "file:///sdcard/ROMs/a.iso"))
        vm.start()
        assertEquals(listOf("org.ppsspp.ppsspp/.PpssppActivity"), platform.prepared)
        assertEquals(listOf("a.e1" to "org.ppsspp.ppsspp"), platform.playedForward)
        assertEquals(listOf("a.e1"), platform.recents)
        assertEquals(LaunchState.Done, vm.state.value)
    }

    @Test
    fun aFrontendTargetThatIsNotSetUpIsRefusedAndNeverPrepared() {
        val vm = launch(request(target = "com.evil.app/.Main"))
        vm.start()
        assertEquals(h.text(R.string.launch_not_allowed, "com.evil.app"), vm.error())
        assertTrue(platform.prepared.isEmpty())
    }

    @Test
    fun aTargetThatCantBeStartedIsAnError() {
        val vm = launch(request(target = "org.ppsspp.ppsspp", data = "file:///sdcard/ROMs/a.iso"))
        platform.forwardOk = false
        vm.start()
        assertEquals(h.text(R.string.launch_not_startable, "PPSSPP app"), vm.error())
        assertTrue(platform.playedForward.isEmpty())
    }

    @Test
    fun aFrontendGameThatFailsShowsWhy() {
        val vm = launch(request(target = "org.ppsspp.ppsspp", data = "file:///sdcard/ROMs/a.iso"))
        platform.playForwardResult = PlayOutcome.Failed("lock held")
        vm.start()
        assertEquals("lock held", vm.error())
        assertTrue(platform.recents.isEmpty())
    }

    // An unknown file: the user says which game it is

    private fun chooser(): LaunchViewModel {
        val vm = launch(request(target = "org.ppsspp.ppsspp", data = "file:///sdcard/ROMs/new.iso"))
        vm.start()
        return vm
    }

    @Test
    fun anUnknownFileAsksWhichGameItIs() {
        val vm = chooser()
        val s = vm.state.value as LaunchState.Choose
        assertEquals("PPSSPP app", s.app)
        assertEquals("new.iso", s.file)
        assertEquals(listOf("a.e1", "b.e1"), s.candidates.map { it.profile.key })
        assertTrue("nothing is played until the user says", platform.playedForward.isEmpty())
    }

    @Test
    fun theChosenGameIsRememberedForTheFileAndPlayed() {
        val vm = chooser()
        val choice = (vm.state.value as LaunchState.Choose).candidates.first { it.profile.key == "b.e1" }
        vm.choose(choice)
        assertTrue("setRom b.e1|file:///sdcard/ROMs/new.iso" in h.core.calls)
        assertEquals(listOf("b.e1" to "org.ppsspp.ppsspp"), platform.playedForward)
        assertEquals(listOf("b.e1"), platform.recents)
        assertEquals(LaunchState.Done, vm.state.value)
    }

    @Test
    fun aFileThatCantBeRememberedStillPlays() {
        val vm = chooser()
        h.core.failing += "setRom"
        vm.choose((vm.state.value as LaunchState.Choose).candidates.first())
        assertEquals(1, platform.playedForward.size)
        assertEquals(listOf("remember file: setRom failed"), platform.logs)
    }

    @Test
    fun aGamesOwnAppRemembersNoFile() {
        // Only an emulator's game has a game file to remember.
        val vm = launch(request(target = "com.chucklefish.stardewvalley"))
        vm.start()
        assertEquals("a game's own app needs no file: played at once", listOf("n" to "com.chucklefish.stardewvalley"), platform.playedForward)
        assertTrue(h.core.calls.none { it.startsWith("setRom") })
    }

    @Test
    fun playingWithoutSyncingStartsTheGameAndCloses() {
        val vm = chooser()
        vm.choose(null)
        assertEquals(1, platform.startedWithoutSync)
        assertEquals(LaunchState.Done, vm.state.value)
        assertTrue(platform.playedForward.isEmpty())
    }

    @Test
    fun aGameThatCantBeStartedWithoutSyncingShowsWhy() {
        val vm = chooser()
        platform.startError = "activity not found"
        vm.choose(null)
        assertEquals(h.text(R.string.launch_failed, "activity not found"), vm.error())
    }

    @Test
    fun choosingBeforeAnythingIsPreparedDoesNothing() {
        val vm = launch(request(key = "a.e1"))
        vm.start()
        platform.playedForward.clear()
        vm.choose(null)
        assertEquals(0, platform.startedWithoutSync)
        assertFalse(platform.playedForward.isNotEmpty())
    }

    // The questions of the sync before playing

    @Test
    fun questionsAreShownAndAnswered() {
        val vm = launch(request())
        h.core.events.tryEmit(CoreEvent.PromptOpened(Prompt(id = "p1", kind = PromptKind.Lock)))
        h.core.events.tryEmit(CoreEvent.PromptOpened(Prompt(id = "p2", kind = PromptKind.Conflict)))
        assertEquals(listOf("p1", "p2"), vm.prompts.value.map { it.id })
        vm.answer(vm.prompts.value.first(), "yes")
        assertEquals(listOf("p1" to "yes"), h.core.responses)
        assertEquals(listOf("p2"), vm.prompts.value.map { it.id })
        h.core.events.tryEmit(CoreEvent.PromptClosed("p2"))
        assertTrue(vm.prompts.value.isEmpty())
    }

    @Test
    fun theProgressMessageShowsWhileWorkingOnly() {
        val vm = launch(request())
        h.core.events.tryEmit(CoreEvent.Status(SessionStatus(message = "Syncing Zelda")))
        assertEquals(LaunchState.Working("Syncing Zelda"), vm.state.value)
        h.core.events.tryEmit(CoreEvent.Status(SessionStatus(message = "")))
        assertEquals("an empty message changes nothing", LaunchState.Working("Syncing Zelda"), vm.state.value)

        val done = launch(request(key = "a.e1"))
        done.start()
        h.core.events.tryEmit(CoreEvent.Status(SessionStatus(message = "late")))
        assertEquals("once finished, a late status doesn't reopen it", LaunchState.Done, done.state.value)
    }
}
