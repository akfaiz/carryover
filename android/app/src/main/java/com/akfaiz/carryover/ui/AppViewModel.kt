package com.akfaiz.carryover.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.AndroidLibrary
import com.akfaiz.carryover.core.CoreApi
import com.akfaiz.carryover.core.CoreEvent
import com.akfaiz.carryover.core.EmulatorSetup
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.PlanView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.Resolution
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.core.SessionStatus
import com.akfaiz.carryover.core.SyncAllProgress
import com.akfaiz.carryover.core.SyncEvent
import com.akfaiz.carryover.data.AppPlatform
import com.akfaiz.carryover.data.AppServices
import com.akfaiz.carryover.data.Conflict
import com.akfaiz.carryover.data.CoverFinder
import com.akfaiz.carryover.data.FindProgress
import com.akfaiz.carryover.data.FindReport
import com.akfaiz.carryover.data.Messenger
import com.akfaiz.carryover.data.SyncController
import com.akfaiz.carryover.data.SyncReport
import com.akfaiz.carryover.data.SyncState
import com.akfaiz.carryover.session.PlayOutcome
import com.akfaiz.carryover.session.Sessions
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the pages show: the library and play state, with what is syncing ([SyncState]). */
data class UiState(
    val initError: String? = null,
    val loaded: Boolean = false,
    /** A cloud is set up (the settings page tells which). */
    val cloudReady: Boolean = false,
    val games: List<GameView> = emptyList(),
    /** Profile keys being synced, with their latest progress. */
    val syncing: Map<String, SyncEvent?> = emptyMap(),
    val syncAll: SyncAllProgress? = null,
    /** The latest progress of the game Sync all is on (bytes of its transfer). */
    val syncAllEvent: SyncEvent? = null,
    val conflict: Conflict? = null,
    val emulators: List<EmulatorView> = emptyList(),
    /** Questions from play sessions waiting for an answer, oldest first. */
    val prompts: List<Prompt> = emptyList(),
    /** Latest play-session status by profile key. */
    val sessions: Map<String, SessionStatus> = emptyMap(),
    /** Profile keys whose game is being started (sync before playing). */
    val starting: Set<String> = emptySet(),
    /** Profile keys whose play session is being ended (uploading). */
    val ending: Set<String> = emptySet(),
    /** A SteamGridDB API key is set: covers and backgrounds can be looked up. */
    val hasGridKey: Boolean = false,
    /** How far the search for the covers of the games without one is; null when none is running. */
    val findingCovers: FindProgress? = null,
    /** The library is being merged with the cloud (pull to refresh). */
    val refreshingLibrary: Boolean = false,
)

/**
 * The library, syncing and playing: what the library, game and emulator
 * pages show. Settings and the cloud are [SettingsViewModel] and
 * [CloudViewModel], over the same [com.akfaiz.carryover.data.SettingsRepository].
 */
class AppViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {
    // The default is what the framework's factory uses (a constructor with no arguments); tests pass their own.

    private val core: CoreApi get() = services.core
    private val m: Messenger get() = services.messenger
    private val platform: AppPlatform get() = services.platform

    private val _state = MutableStateFlow(
        UiState(
            initError = core.initError,
            cloudReady = services.settings.state.value.cloudReady,
            hasGridKey = services.settings.state.value.hasGridKey,
        ),
    )
    private val syncs = SyncController(services.core)
    private val finder = CoverFinder(services.core)

    val state: StateFlow<UiState> = combine(_state, syncs.state, finder.state) { s, y, f -> s.withSync(y).copy(findingCovers = f) }
        .stateIn(viewModelScope, SharingStarted.Eagerly, _state.value.withSync(syncs.state.value))

    /** One-off messages for a snackbar. */
    val messages get() = m.messages

    private var reload: Job? = null

    init {
        if (core.initError == null) {
            viewModelScope.launch { core.events.collect(::onEvent) }
            viewModelScope.launch {
                platform.ending.collect { ending -> _state.update { it.copy(ending = ending) } }
            }
            viewModelScope.launch {
                services.settings.state.map { it.cloudReady }.distinctUntilChanged()
                    .collect { ready -> _state.update { it.copy(cloudReady = ready) } }
            }
            viewModelScope.launch {
                services.settings.state.map { it.hasGridKey }.distinctUntilChanged()
                    .collect { has -> _state.update { it.copy(hasGridKey = has) } }
            }
            // A cloud was connected: the library came from it.
            viewModelScope.launch { services.settings.libraryStale.collect { scheduleReload() } }
            viewModelScope.launch {
                val waiting = runCatching { core.prompts() }.getOrDefault(emptyList())
                _state.update { it.copy(prompts = (it.prompts + waiting).distinctBy { p -> p.id }) }
            }
            refresh()
            announceUpdate()
        }
    }

    /** Says there is a newer version (at most every 12 hours does the core ask for it; off in the settings). */
    private fun announceUpdate() {
        viewModelScope.launch {
            val st = runCatching { core.checkUpdate(false) }.getOrNull() ?: return@launch
            if (st.newer) m.say(m.str(R.string.msg_update_available, st.latest))
        }
    }

    private fun UiState.withSync(y: SyncState) =
        copy(syncing = y.syncing, syncAll = y.syncAll, syncAllEvent = y.syncAllEvent, conflict = y.conflict)

    private fun onEvent(ev: CoreEvent) {
        when (ev) {
            is CoreEvent.Library -> scheduleReload()
            is CoreEvent.Sync -> syncs.onSync(ev.event)
            is CoreEvent.SyncAll -> syncs.onSyncAll(ev.progress)
            is CoreEvent.Status -> _state.update {
                val key = ev.status.key.ifEmpty { ev.status.gameId }
                it.copy(sessions = it.sessions + (key to ev.status))
            }
            is CoreEvent.PromptOpened -> _state.update { it.copy(prompts = it.prompts + ev.prompt) }
            is CoreEvent.PromptClosed -> _state.update { it.copy(prompts = it.prompts.filter { p -> p.id != ev.id }) }
            is CoreEvent.Update -> Unit // the settings page follows it
        }
    }

    /** Coalesces bursts of library events into one reload. */
    private fun scheduleReload() {
        reload?.cancel()
        reload = viewModelScope.launch {
            delay(150)
            loadGames()
        }
    }

    private suspend fun loadGames() {
        try {
            val library = AndroidLibrary.of(core.games(), core.emulators())
            _state.update { it.copy(games = library.games, emulators = library.emulators, loaded = true) }
        } catch (e: Exception) {
            m.failed(m.str(R.string.err_read_library), e)
        }
    }

    fun refresh() {
        viewModelScope.launch {
            m.attempt(m.str(R.string.err_read_settings)) { services.settings.refresh() }
            loadGames()
        }
    }

    /**
     * Pull to refresh: merges the library with the cloud (games, emulators and
     * covers added on other devices). No saves are synced: that is Sync all.
     */
    fun refreshLibrary() {
        if (_state.value.refreshingLibrary) return
        _state.update { it.copy(refreshingLibrary = true) }
        viewModelScope.launch {
            try {
                m.attempt(m.str(R.string.err_library_refresh)) { services.settings.refreshLibrary() }
                loadGames()
            } finally {
                _state.update { it.copy(refreshingLibrary = false) }
            }
        }
    }

    /**
     * Back in the foreground: look for saves changed while away, and let the
     * session watcher know (without Usage access, coming back to Carryover is
     * how it learns the game was left; it also picks up sessions a killed
     * process left open).
     */
    fun onResume() {
        if (core.initError != null) return
        viewModelScope.launch {
            runCatching { core.checkLocalChanges() }
            if (runCatching { core.openSessions() }.getOrDefault(emptyList()).isNotEmpty()) {
                platform.appResumed()
            }
        }
    }

    // Play

    /**
     * The app that plays a profile on this device, as (exe, args, rom), or null
     * when none is set up: an emulator's app, or the game's own app.
     */
    fun launchSetup(p: Profile): Triple<String, String, String>? = Sessions.setup(p, _state.value.emulators)

    /**
     * Plays a profile: syncs before playing (questions arrive as prompts), takes
     * the cloud lock, starts the app and hands the session to the watcher,
     * which uploads the saves once the user has left the game.
     */
    fun play(g: GameView, p: Profile) {
        if (p.key in _state.value.starting) return
        val title = profileTitle(g, p)
        _state.update { it.copy(starting = it.starting + p.key) }
        viewModelScope.launch {
            try {
                platform.play(p.key, p.emulatorId == null, title, p, _state.value.emulators).fold(
                    onSuccess = { out ->
                        when (out) {
                            is PlayOutcome.Cancelled -> m.say(m.str(R.string.msg_not_started, title))
                            is PlayOutcome.Failed -> m.say(out.message)
                            is PlayOutcome.Started -> {
                                platform.rememberRecent(p.key, title, g.coverPath)
                                if (out.offline) m.say(m.str(R.string.msg_offline_playing, title))
                            }
                        }
                    },
                    // The game can't be started on this device: not set up, not installed.
                    onFailure = { m.say(m.str(R.string.msg_error, title, it.message.orEmpty())) },
                )
            } finally {
                _state.update { it.copy(starting = it.starting - p.key) }
            }
        }
    }

    /** Moves a game to the trash on all devices (its cloud saves stay); [done] when it is. */
    fun deleteGame(id: String, name: String, done: () -> Unit = {}) {
        viewModelScope.launch {
            try {
                core.deleteGame(id)
                m.say(m.str(R.string.msg_game_trashed, name))
                loadGames()
                done()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_delete_game, name), e)
            }
        }
    }

    /** Moves an emulator to the trash on all devices; [done] when it is. */
    fun deleteEmulator(id: String, name: String, done: () -> Unit = {}) {
        viewModelScope.launch {
            try {
                core.deleteEmulator(id)
                m.say(m.str(R.string.msg_emulator_trashed, name))
                loadGames()
                done()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_delete_game, name), e)
            }
        }
    }

    /** Pins a home-screen shortcut that plays a profile (through LaunchActivity). */
    fun addShortcut(g: GameView, p: Profile) {
        if (!platform.pinShortcut(p.key, profileTitle(g, p), g.coverPath)) m.say(m.str(R.string.msg_no_shortcuts))
    }

    /** Ends a play session now (the user is done), uploading its saves. */
    fun donePlaying(key: String) = platform.donePlaying(key)

    fun answer(prompt: Prompt, answer: String) {
        core.respond(prompt.id, answer)
        _state.update { it.copy(prompts = it.prompts.filter { p -> p.id != prompt.id }) }
    }

    fun setRom(key: String, rom: String) {
        viewModelScope.launch {
            try {
                core.setRom(key, rom)
                m.say(m.str(if (rom.isBlank()) R.string.msg_game_file_cleared else R.string.msg_game_file_saved))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_save_game_file), e)
            }
        }
    }

    fun setNativeApp(gameId: String, pkg: String) {
        viewModelScope.launch {
            try {
                core.setNativeApp(gameId, pkg)
                m.say(m.str(R.string.msg_app_saved))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_save_app), e)
            }
        }
    }

    /** Removes this device's setup of an emulator (no longer used here). */
    fun unsetEmulator(id: String, name: String) {
        viewModelScope.launch {
            try {
                core.unsetEmulator(id)
                m.say(m.str(R.string.msg_emulator_removed, name))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_remove_emulator, name), e)
            }
        }
    }

    /** Saves this device's setup of an emulator (new when id is empty); [done] gets its id. */
    fun saveEmulator(id: String, name: String, exe: String, args: String, dataDir: String, done: (String) -> Unit = {}) {
        viewModelScope.launch {
            try {
                val preset = AndroidLibrary.presetFor(_state.value.emulators.firstOrNull { it.id == id }, exe.trim())
                val platforms = AndroidLibrary.platformsFor(exe.trim())
                val saved = core.saveEmulator(EmulatorSetup(id, name.trim(), exe.trim(), args.trim(), preset, dataDir.trim(), platforms))
                m.say(m.str(R.string.msg_emulator_saved, name))
                loadGames()
                done(saved.id)
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_save_emulator, name), e)
            }
        }
    }

    // Sync

    fun sync(key: String, name: String, resolution: Resolution? = null) {
        viewModelScope.launch { syncs.sync(key, name, resolution)?.let(::tell) }
    }

    fun cancelSync(key: String) = syncs.cancel(key)

    fun resolveConflict(keep: Resolution?) {
        val c = syncs.takeConflict() ?: return
        if (keep != null) sync(c.key, c.name, keep)
    }

    fun syncAll() {
        viewModelScope.launch {
            val report = syncs.syncAll() ?: return@launch
            tell(report)
            loadGames()
        }
    }

    /** Says how a sync went. */
    private fun tell(r: SyncReport) {
        when (r) {
            is SyncReport.Offline -> m.say(m.str(R.string.msg_sync_offline, r.name))
            is SyncReport.Cancelled -> m.say(m.str(R.string.msg_sync_cancelled, r.name))
            is SyncReport.Pushed -> m.say(m.str(R.string.msg_sync_pushed, r.name))
            is SyncReport.Pulled -> m.say(m.str(R.string.msg_sync_pulled, r.name))
            is SyncReport.UpToDate -> m.say(m.str(R.string.msg_sync_none, r.name))
            is SyncReport.NeedsDecision -> {} // the conflict dialog says it
            is SyncReport.Failed -> m.failed(m.str(R.string.err_sync, r.name), r.cause)
            SyncReport.AllOffline -> m.say(m.str(R.string.msg_syncall_offline))
            SyncReport.AllCancelled -> m.say(m.str(R.string.msg_syncall_cancelled))
            is SyncReport.AllFailed -> m.failed(m.str(R.string.err_sync_all), r.cause)
            is SyncReport.AllDone -> m.say(
                when {
                    r.failed.isNotEmpty() -> m.plural(
                        R.plurals.msg_syncall_failed,
                        r.failed.size,
                        r.failed.size,
                        r.failed.first().name,
                        r.failed.first().error,
                    )
                    r.conflicts > 0 -> m.plural(R.plurals.msg_syncall_conflicts, r.conflicts, r.conflicts)
                    else -> m.plural(R.plurals.msg_syncall_done, r.results.size, r.results.size)
                },
            )
        }
    }

    // Library

    fun addGame(name: String, path: String, done: (String) -> Unit) {
        viewModelScope.launch {
            try {
                val g = core.addGame(name.trim(), path.trim())
                loadGames()
                done(g.id)
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_add_game), e)
            }
        }
    }

    /** Adds a game played with an emulator and opens it (done gets its id). */
    fun addEmulatorGame(name: String, emulatorId: String, rom: String, savePath: String, done: (String) -> Unit) {
        viewModelScope.launch {
            try {
                val g = core.addEmulatorGame(name.trim(), emulatorId, rom.trim(), savePath.trim())
                loadGames()
                done(g.id)
                // The folder was left empty: the emulator's manifest may know where this game's saves are.
                if (savePath.isBlank()) autofillSaves(g)
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_add_game), e)
            }
        }
    }

    /**
     * Fills in the new game's save folder with what the emulator's manifest finds on this device. It was not asked
     * for, so a lookup that fails (no type or data folder yet, no network for the manifest) says nothing.
     */
    private suspend fun autofillSaves(g: GameView) {
        val profile = g.emulators.firstOrNull() ?: return
        val applied = try {
            core.autofillSaves(profile.key)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            return
        }
        if (applied > 0) {
            m.say(m.str(R.string.msg_suggest_autofilled))
            loadGames()
        }
    }

    /** Adds an Android game: its own app and its save folder. */
    fun addAndroidGame(name: String, pkg: String, savePath: String, done: (String) -> Unit) {
        viewModelScope.launch {
            try {
                val g = core.addGame(name.trim(), savePath.trim())
                core.setNativeApp(g.id, pkg)
                loadGames()
                done(g.id)
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_add_game), e)
            }
        }
    }

    suspend fun guessRomName(emulatorId: String, fileName: String): String =
        runCatching { core.guessRomName(emulatorId, fileName) }.getOrDefault("")

    /** The whole game, with the emulators used only on other devices too (the library leaves those out). */
    suspend fun fullGame(id: String): GameView? = try {
        core.game(id)
    } catch (e: Exception) {
        if (e is CancellationException) throw e
        null
    }

    fun renameGame(gameId: String, name: String) = edit {
        core.renameGame(gameId, name.trim())
        m.str(R.string.msg_game_renamed)
    }

    fun addGameEmulator(gameId: String, emulatorId: String, shareWith: String, emulatorName: String) = edit {
        core.addGameEmulator(gameId, emulatorId, shareWith)
        m.str(R.string.msg_game_emulator_added, emulatorName)
    }

    fun removeGameEmulator(gameId: String, emulatorId: String, emulatorName: String) = edit {
        core.removeGameEmulator(gameId, emulatorId)
        m.str(R.string.msg_game_emulator_removed, emulatorName)
    }

    /** Sets a profile's save locations ([CoreApi.setSlots]); the core refuses a new one with no name. */
    fun setSlots(key: String, slots: List<SaveSlot>) = edit {
        core.setSlots(key, slots)
        m.str(R.string.msg_slots_saved)
    }

    /** Runs a change to a game, says how it went and reloads the library. */
    private fun edit(change: suspend () -> String) {
        viewModelScope.launch {
            try {
                m.say(change())
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_edit_game), e)
            }
        }
    }

    fun setPaths(key: String, paths: Map<String, String>) {
        viewModelScope.launch {
            try {
                core.setPaths(key, paths)
                m.say(m.str(R.string.msg_paths_saved))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_save_paths), e)
            }
        }
    }

    /** Says a value was copied (Android 13+ shows its own confirmation). */
    fun copied() = m.say(m.str(R.string.msg_copied))

    // Covers

    /**
     * Makes an image the user picked the game's cover. It is read through its
     * content:// URI, scaled to at most 900 px tall (a Steam cover's size) and
     * handed to the core as a JPEG, which shares it with the other devices.
     */
    fun setCover(gameId: String, uri: String) {
        viewModelScope.launch {
            try {
                val file = platform.prepareCover(uri)
                try {
                    core.setCover(gameId, file.path)
                } finally {
                    file.delete()
                }
                m.say(m.str(R.string.msg_cover_changed))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_cover), e)
            }
        }
    }

    /** Finds a cover on SteamGridDB for each game without one (the best match; the game page lets you choose). */
    fun findCovers() {
        viewModelScope.launch {
            val report = finder.findMissing(_state.value.games) ?: return@launch
            when (report) {
                FindReport.NothingToDo -> m.say(m.str(R.string.msg_covers_none))
                is FindReport.Done -> m.say(m.str(R.string.msg_covers_found, report.found, report.total))
                is FindReport.Failed -> {
                    if (report.found > 0) m.say(m.str(R.string.msg_covers_found, report.found, report.total))
                    m.failed(m.str(R.string.err_find_covers), report.cause)
                }
            }
            loadGames()
        }
    }

    /** Removes the background of a game's page. */
    fun clearBackground(id: String) {
        viewModelScope.launch {
            try {
                core.clearBackground(id)
                m.say(m.str(R.string.msg_background_removed))
                loadGames()
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_remove_background), e)
            }
        }
    }

    /** What a sync would do (for a conflict: what each side has); null if the cloud can't tell. */
    suspend fun plan(key: String): PlanView? = try {
        core.plan(key)
    } catch (e: Exception) {
        if (e is CancellationException) throw e
        null
    }
}
