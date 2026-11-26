package com.akfaiz.carryover.ui

import com.akfaiz.carryover.core.ArtCore
import com.akfaiz.carryover.core.Backup
import com.akfaiz.carryover.core.BackupCore
import com.akfaiz.carryover.core.BackupKind
import com.akfaiz.carryover.core.ArtKind
import com.akfaiz.carryover.core.CoreApi
import com.akfaiz.carryover.core.CoreEvent
import com.akfaiz.carryover.core.DeviceConfig
import com.akfaiz.carryover.core.EmulatorSetup
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.GridArt
import com.akfaiz.carryover.core.GridGame
import com.akfaiz.carryover.core.ImportResult
import com.akfaiz.carryover.core.Leftover
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.core.Provider
import com.akfaiz.carryover.core.Question
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.core.SaveSuggestion
import com.akfaiz.carryover.core.SaveSuggestions
import com.akfaiz.carryover.core.SessionCore
import com.akfaiz.carryover.core.SuggestCore
import com.akfaiz.carryover.core.SettingsCore
import com.akfaiz.carryover.core.SyncOutcome
import com.akfaiz.carryover.core.TrashCore
import com.akfaiz.carryover.core.TrashView
import com.akfaiz.carryover.core.UpdateCore
import com.akfaiz.carryover.core.UpdateState
import com.akfaiz.carryover.core.UpdateStatus
import com.akfaiz.carryover.core.SyncCore
import com.akfaiz.carryover.data.AppPlatform
import com.akfaiz.carryover.data.AppServices
import com.akfaiz.carryover.data.FakePrefs
import com.akfaiz.carryover.data.FakeSettingsCore
import com.akfaiz.carryover.data.FakeSyncCore
import com.akfaiz.carryover.data.Messenger
import com.akfaiz.carryover.data.SettingsRepository
import com.akfaiz.carryover.data.Texts
import com.akfaiz.carryover.session.FakeCore
import com.akfaiz.carryover.session.PlayOutcome
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestCoroutineScheduler
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import java.io.File

/** Texts as "id:arg|arg", so a test says which text of the catalog a message is, in any language. */
object FakeTexts : Texts {
    override fun string(id: Int, vararg args: Any): String = "$id:" + args.joinToString("|")
    override fun plural(id: Int, count: Int, vararg args: Any): String = "p$id#$count:" + args.joinToString("|")
}

/** The trash and the leftover cloud folders of the core: what it holds, and every call recorded. */
internal class FakeTrashCore : TrashCore {
    var view = TrashView()
    var leftoverList: List<Leftover> = emptyList()
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()

    /** When set, the deletes for good wait for it: they are "running" (they can take a while in the cloud). */
    var gate: CompletableDeferred<Unit>? = null

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    override suspend fun trash(): TrashView {
        call("trash")
        return view
    }

    override suspend fun deleteGame(id: String) = call("deleteGame", id)

    override suspend fun restoreGame(id: String) {
        call("restoreGame", id)
        view = view.copy(games = view.games.filter { it.id != id })
    }

    override suspend fun deleteGameForever(id: String, withSaves: Boolean) {
        call("deleteGameForever", "$id|$withSaves")
        gate?.await()
        view = view.copy(games = view.games.filter { it.id != id })
    }

    override suspend fun deleteEmulator(id: String) = call("deleteEmulator", id)

    override suspend fun restoreEmulator(id: String) {
        call("restoreEmulator", id)
        view = view.copy(emulators = view.emulators.filter { it.id != id })
    }

    override suspend fun deleteEmulatorForever(id: String, withSaves: Boolean) {
        call("deleteEmulatorForever", "$id|$withSaves")
        gate?.await()
        view = view.copy(emulators = view.emulators.filter { it.id != id })
    }

    /** When set, looking in the cloud waits for it. */
    var leftoversGate: CompletableDeferred<Unit>? = null

    override suspend fun leftovers(): List<Leftover> {
        call("leftovers")
        leftoversGate?.await()
        return leftoverList
    }

    override suspend fun restoreLeftover(key: String): String {
        call("restoreLeftover", key)
        leftoverList = leftoverList.filter { it.key != key }
        return "restored-id"
    }

    override suspend fun deleteLeftover(key: String) {
        call("deleteLeftover", key)
        leftoverList = leftoverList.filter { it.key != key }
    }
}

/** SteamGridDB: what it knows, and every call recorded. */
internal class FakeArtCore : ArtCore {
    /** SteamGridDB games by the search term. */
    var games: Map<String, List<GridGame>> = emptyMap()

    /** Covers by SteamGridDB game id. */
    var covers: Map<Int, List<GridArt>> = emptyMap()

    /** Backgrounds by SteamGridDB game id. */
    var backgrounds: Map<Int, List<GridArt>> = emptyMap()
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()

    /** Search terms that fail. */
    val failTerms = mutableSetOf<String>()

    /** When set, a search waits for it. */
    var gate: CompletableDeferred<Unit>? = null

    /** When set, downloading an image waits for it. */
    var applyGate: CompletableDeferred<Unit>? = null

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    override suspend fun searchGridGames(term: String): List<GridGame> {
        call("search", term)
        gate?.await()
        if (term in failTerms) throw IllegalStateException("search $term failed")
        return games[term].orEmpty()
    }

    override suspend fun gridArt(kind: ArtKind, gridGameId: Int): List<GridArt> {
        call("art", "${kind.wire}|$gridGameId")
        gate?.await()
        return (if (kind == ArtKind.Cover) covers else backgrounds)[gridGameId].orEmpty()
    }

    override suspend fun setCoverFromUrl(gameId: String, url: String): GameView {
        call("setCover", "$gameId|$url")
        applyGate?.await()
        return GameView(id = gameId)
    }

    override suspend fun setBackgroundFromUrl(gameId: String, url: String): GameView {
        call("setBackground", "$gameId|$url")
        applyGate?.await()
        return GameView(id = gameId)
    }

    override suspend fun clearBackground(gameId: String): GameView {
        call("clearBackground", gameId)
        return GameView(id = gameId)
    }
}

/** The backups of the core by profile key, and every call recorded. */
internal class FakeBackupCore : BackupCore {
    var backups: Map<String, List<Backup>> = emptyMap()
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()

    /** What "Back up now" and a restore end as (offline, cancelled); the default is done. */
    var outcome = SyncOutcome()

    /** When set, "Back up now" and a restore wait for it: they are "running". */
    var gate: CompletableDeferred<Unit>? = null

    /** When set, listing the backups waits for it. */
    var listGate: CompletableDeferred<Unit>? = null

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    override suspend fun backups(key: String): List<Backup> {
        call("backups", key)
        val list = backups[key].orEmpty()
        listGate?.await()
        return list
    }

    override suspend fun backupNow(key: String): SyncOutcome {
        call("backupNow", key)
        gate?.await()
        if (!outcome.offline && !outcome.cancelled) {
            val made = Backup(name = "made-${calls.size}", time = "2026-11-22T10:00:00Z", deviceName = "Pixel", kind = BackupKind.Manual, pinned = true)
            backups += key to (listOf(made) + backups[key].orEmpty())
        }
        return outcome
    }

    override suspend fun setBackupPinned(key: String, name: String, pinned: Boolean) {
        call("pin", "$key|$name|$pinned")
        backups += key to backups[key].orEmpty().map { if (it.name == name) it.copy(pinned = pinned) else it }
    }

    override suspend fun deleteBackup(key: String, name: String) {
        call("delete", "$key|$name")
        backups += key to backups[key].orEmpty().filter { it.name != name }
    }

    override suspend fun restoreBackup(key: String, name: String): SyncOutcome {
        call("restore", "$key|$name")
        gate?.await()
        return outcome
    }
}

/** Where the emulators of the core keep saves: what it finds, and every call recorded. */
internal class FakeSuggestCore : SuggestCore {
    var found = SaveSuggestions()
    var applied = 0
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()

    /** When set, a lookup waits for it. */
    var gate: CompletableDeferred<Unit>? = null

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    override suspend fun suggestSaves(key: String, rom: String): SaveSuggestions {
        call("suggest", "$key|$rom")
        val answer = found // what it knew when asked, however long it takes
        gate?.await()
        return answer
    }

    override suspend fun useSaveSuggestion(key: String, suggestion: SaveSuggestion): GameView {
        call("use", "$key|${suggestion.path}")
        return GameView(id = key.substringBefore('.'))
    }

    override suspend fun autofillSaves(key: String, rom: String): Int {
        call("autofill", "$key|$rom")
        return applied
    }
}

/** Updating the app: what the core knows, and every call recorded. */
internal class FakeUpdateCore : UpdateCore {
    var state = UpdateState()

    /** What a download ends as. */
    var downloaded = UpdateState(status = UpdateStatus.Ready, latest = "1.1.0", file = "/cache/updates/app.apk")
    var busy = false
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    override suspend fun updateState(): UpdateState = state

    override suspend fun checkUpdate(force: Boolean): UpdateState {
        call("check", "$force")
        return state
    }

    override suspend fun downloadUpdate(): UpdateState {
        call("download")
        state = downloaded
        return state
    }

    override fun cancelUpdate(): Boolean {
        calls += "cancel"
        return true
    }

    override suspend fun updateBusy(): Boolean = busy
}

/** The whole Go bridge as a fake: the partial fakes of each area, and the library calls recorded here. */
internal class FakeCoreApi(
    val session: FakeCore = FakeCore(),
    val settingsCore: FakeSettingsCore = FakeSettingsCore(),
    val sync: FakeSyncCore = FakeSyncCore(),
    val trash: FakeTrashCore = FakeTrashCore(),
    val art: FakeArtCore = FakeArtCore(),
    val backup: FakeBackupCore = FakeBackupCore(),
    val suggest: FakeSuggestCore = FakeSuggestCore(),
    val updates: FakeUpdateCore = FakeUpdateCore(),
) : CoreApi, UpdateCore by updates, SessionCore by session, SettingsCore by settingsCore, SyncCore by sync, BackupCore by backup, SuggestCore by suggest, TrashCore by trash, ArtCore by art {
    override var initError: String? = null
    override val events = MutableSharedFlow<CoreEvent>(extraBufferCapacity = 64)
    override val authUrl = MutableStateFlow<String?>(null)
    override val version = "test-core"

    /** Every library call, as "name detail". */
    val calls = mutableListOf<String>()

    /** Names of the calls that fail with "<name> failed". */
    val failing = mutableSetOf<String>()
    var prompts: List<Prompt> = emptyList()
    val responses = mutableListOf<Pair<String, String>>()
    var providers: List<Provider> = emptyList()
    var question = Question(done = true)

    private fun call(name: String, detail: String = "") {
        calls += "$name $detail".trim()
        if (name in failing) throw IllegalStateException("$name failed")
    }

    private fun gameView(id: String, name: String = id) = GameView(id = id, name = name)

    override fun openBrowser(url: String) {
        calls += "openBrowser $url"
    }

    override fun clearAuthUrl() {
        authUrl.value = null
    }

    override suspend fun providers(): List<Provider> {
        call("providers")
        return providers
    }

    override suspend fun createRemote(name: String, type: String, params: Map<String, String>): Question {
        call("createRemote", "$name|$type|${params.entries.joinToString(",") { "${it.key}=${it.value}" }}")
        return question
    }

    /** What importing an rclone.conf ends as, or the error it fails with. */
    var importResult = ImportResult()
    var importError: Exception? = null

    override suspend fun importRcloneConfig(contents: String): ImportResult {
        call("importConfig", contents)
        importError?.let { throw it }
        return importResult
    }

    override suspend fun continueRemote(name: String, state: String, answer: String): Question {
        call("continueRemote", "$name|$state|$answer")
        return question
    }

    override suspend fun cancelSignIn(): Boolean {
        call("cancelSignIn")
        return true
    }

    override suspend fun game(id: String): GameView {
        call("game", id)
        return session.games.firstOrNull { it.id == id } ?: gameView(id)
    }

    override suspend fun addGame(name: String, path: String): GameView {
        call("addGame", "$name|$path")
        return gameView("new-id", name)
    }

    override suspend fun addEmulatorGame(name: String, emulatorId: String, rom: String, savePath: String): GameView {
        call("addEmulatorGame", "$name|$emulatorId|$rom|$savePath")
        return gameView("emu-id", name).copy(emulators = listOf(ProfileView(emulatorId = emulatorId, key = "emu-id.$emulatorId")))
    }

    override suspend fun renameGame(gameId: String, name: String): GameView {
        call("renameGame", "$gameId|$name")
        return gameView(gameId, name)
    }

    override suspend fun addGameEmulator(gameId: String, emulatorId: String, shareWith: String): GameView {
        call("addGameEmulator", "$gameId|$emulatorId|$shareWith")
        return gameView(gameId)
    }

    override suspend fun removeGameEmulator(gameId: String, emulatorId: String): GameView {
        call("removeGameEmulator", "$gameId|$emulatorId")
        return gameView(gameId)
    }

    override suspend fun setSlots(key: String, slots: List<SaveSlot>): GameView {
        call("setSlots", "$key|${slots.joinToString(",") { "${it.key}=${it.label}" }}")
        return gameView(key)
    }

    override suspend fun guessRomName(emulatorId: String, fileName: String): String {
        call("guessRomName", "$emulatorId|$fileName")
        return "Guessed"
    }

    override suspend fun setPaths(key: String, paths: Map<String, String>): GameView {
        call("setPaths", "$key|${paths.entries.joinToString(",") { "${it.key}=${it.value}" }}")
        return gameView(key)
    }

    override suspend fun setRom(key: String, rom: String): GameView {
        call("setRom", "$key|$rom")
        return gameView(key)
    }

    override suspend fun setNativeApp(gameId: String, pkg: String): GameView {
        call("setNativeApp", "$gameId|$pkg")
        return gameView(gameId)
    }

    override suspend fun setCover(gameId: String, path: String): GameView {
        call("setCover", "$gameId|${File(path).name}")
        return gameView(gameId)
    }

    override suspend fun unsetEmulator(id: String) {
        call("unsetEmulator", id)
    }

    val savedEmulators = mutableListOf<EmulatorSetup>()

    override suspend fun saveEmulator(setup: EmulatorSetup): EmulatorView {
        call("saveEmulator", setup.id)
        savedEmulators += setup
        return EmulatorView(id = setup.id.ifEmpty { "new-emulator" }, name = setup.name)
    }

    override suspend fun previewPath(gameId: String, raw: String): String = raw

    override suspend fun checkLocalChanges() {
        call("checkLocalChanges")
    }

    override suspend fun waitBackground() {}

    override fun respond(promptId: String, answer: String): Boolean {
        responses += promptId to answer
        return true
    }

    override suspend fun prompts(): List<Prompt> = prompts
}

/** The Android parts the view models use, recorded. */
internal class FakeAppPlatform : AppPlatform {
    override val ending = MutableStateFlow<Set<String>>(emptySet())

    var playResult: Result<PlayOutcome> = Result.success(PlayOutcome.Started(false))

    /** When set, [play] waits for it: the game is "starting". */
    var playGate: CompletableDeferred<Unit>? = null
    val played = mutableListOf<String>()
    val recents = mutableListOf<Triple<String, String, String>>()
    var canPin = true
    val pinned = mutableListOf<Triple<String, String, String>>()
    val done = mutableListOf<String>()
    var resumed = 0
    var mayInstall = true
    var canOpenInstaller = true
    var permissionAsked = 0
    val installed = mutableListOf<String>()
    var cover: File? = null
    var coverError: Exception? = null
    var logs: File? = null
    var logsError: Exception? = null
    val savedLogs = mutableListOf<Pair<File, String>>()
    var saveLogsError: Exception? = null

    override suspend fun play(key: String, native: Boolean, title: String, p: Profile, emulators: List<EmulatorView>): Result<PlayOutcome> {
        played += "$key|$native|$title"
        playGate?.await()
        return playResult
    }

    override fun rememberRecent(key: String, title: String, coverPath: String) {
        recents += Triple(key, title, coverPath)
    }

    override fun pinShortcut(key: String, title: String, coverPath: String): Boolean {
        pinned += Triple(key, title, coverPath)
        return canPin
    }

    override fun donePlaying(key: String) {
        done += key
    }

    override fun appResumed() {
        resumed++
    }

    override suspend fun prepareCover(uri: String): File {
        coverError?.let { throw it }
        return cover ?: File.createTempFile("cover", ".jpg")
    }

    override fun canInstallPackages(): Boolean = mayInstall

    override fun openInstallPermission(): Boolean {
        permissionAsked++
        return true
    }

    override fun installApk(path: String): Boolean {
        if (canOpenInstaller) installed += path
        return canOpenInstaller
    }

    override suspend fun buildLogs(): File {
        logsError?.let { throw it }
        return logs ?: File.createTempFile("logs", ".zip")
    }

    override suspend fun saveLogs(zip: File, target: String) {
        saveLogsError?.let { throw it }
        savedLogs += zip to target
    }
}

/**
 * What a view model test needs: the fakes, the services made of them, the
 * messages the user was told, and a test Main dispatcher (viewModelScope runs
 * on it; unconfined, so work finishes inside the call; delays are virtual).
 * Call [close] when the test ends.
 */
@OptIn(ExperimentalCoroutinesApi::class)
internal class Harness {
    val scheduler = TestCoroutineScheduler()
    private val dispatcher = UnconfinedTestDispatcher(scheduler)

    init {
        Dispatchers.setMain(dispatcher)
    }

    val core = FakeCoreApi()
    val prefs = FakePrefs()
    val platform = FakeAppPlatform()
    val settings = SettingsRepository(core, prefs) {}
    val messenger = Messenger(FakeTexts)
    val services = AppServices(core, settings, messenger, platform)

    /** The messages said so far, oldest first. */
    val said = mutableListOf<String>()

    init {
        CoroutineScope(dispatcher).launch { messenger.messages.collect { said += it } }
    }

    fun close() {
        Dispatchers.resetMain()
    }

    /** Moves the virtual clock (the library reload waits 150 ms). */
    fun advance(ms: Long) {
        scheduler.advanceTimeBy(ms)
        scheduler.runCurrent()
    }

    fun text(id: Int, vararg args: Any): String = FakeTexts.string(id, *args)
    fun plural(id: Int, n: Int, vararg args: Any): String = FakeTexts.plural(id, n, *args)
}

/** A profile of [key] that can sync (its save folder is set). */
internal fun profileOf(key: String = "g", emulator: String? = null, rom: String = "", app: String = ""): Profile = Profile(
    key = key,
    emulator = emulator,
    emulatorId = emulator?.lowercase(),
    slots = listOf(SaveSlot("saves")),
    paths = mapOf("saves" to "/x"),
    resolvedPaths = mapOf("saves" to "/x"),
    pathErrors = emptyMap(),
    status = GameStatus(),
    rom = rom,
    app = app,
)

internal fun gameOf(id: String = "g", name: String = "Zelda", cover: String = ""): GameView = GameView(
    id = id,
    name = name,
    slots = listOf(SaveSlot("saves")),
    device = DeviceConfig(paths = mapOf("saves" to "/x"), exe = "com.example.game"),
    coverPath = cover,
)
