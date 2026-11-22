package com.akfaiz.carryover.core

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.os.SystemClock
import android.util.Log
import com.akfaiz.carryover.bridge.mobile.Host
import com.akfaiz.carryover.bridge.mobile.Mobile
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import java.io.File

/** Something the Go core reported. */
sealed interface CoreEvent {
    /** The library changed; gameId is "" for a change to all of it. */
    data class Library(val gameId: String) : CoreEvent
    data class Sync(val event: SyncEvent) : CoreEvent
    data class SyncAll(val progress: SyncAllProgress) : CoreEvent
    data class Status(val status: SessionStatus) : CoreEvent
    data class PromptOpened(val prompt: Prompt) : CoreEvent
    data class PromptClosed(val id: String) : CoreEvent
}

/**
 * The Go core (package mobile, bound with gomobile). Calls run on
 * [Dispatchers.IO]; Go errors arrive as exceptions whose message is the error.
 */
object Core : CoreApi {
    private const val TAG = "Carryover"

    private lateinit var app: Context

    /** Why the core couldn't start, or null when it runs. */
    override var initError: String? = null
        private set

    private val _events = MutableSharedFlow<CoreEvent>(extraBufferCapacity = 256)
    override val events: SharedFlow<CoreEvent> = _events

    /** The authorisation page of the cloud provider being connected, if any. */
    private val _authUrl = MutableStateFlow<String?>(null)
    override val authUrl: StateFlow<String?> = _authUrl

    fun init(context: Context) {
        app = context.applicationContext
        val started = SystemClock.elapsedRealtime()
        try {
            Mobile.init(
                File(app.filesDir, "carryover").path,
                app.cacheDir.path,
                Environment.getExternalStorageDirectory().path,
                deviceName(),
                host,
            )
            // Save folders granted through the Storage Access Framework.
            Mobile.setStorage(SafStorage(app))
            Log.i(TAG, "core init took ${SystemClock.elapsedRealtime() - started} ms")
        } catch (e: Exception) {
            Log.e(TAG, "core init", e)
            initError = e.message ?: e.toString()
        }
    }

    override val version: String get() = Mobile.version()

    private fun deviceName(): String {
        val maker = Build.MANUFACTURER.orEmpty().replaceFirstChar { it.uppercase() }
        val model = Build.MODEL.orEmpty()
        return if (model.startsWith(maker, ignoreCase = true)) model else "$maker $model".trim()
    }

    private val host = object : Host {
        override fun openURL(url: String) {
            _authUrl.value = url
            openBrowser(url)
        }

        override fun event(kind: String, payload: String) {
            val ev = try {
                when (kind) {
                    Mobile.EventLibrary -> CoreEvent.Library(payload)
                    Mobile.EventSync -> CoreEvent.Sync(json.decodeFromString(payload))
                    Mobile.EventSyncAll -> CoreEvent.SyncAll(json.decodeFromString(payload))
                    Mobile.EventStatus -> CoreEvent.Status(json.decodeFromString(payload))
                    Mobile.EventPrompt -> CoreEvent.PromptOpened(json.decodeFromString(payload))
                    Mobile.EventPromptClosed -> CoreEvent.PromptClosed(payload)
                    else -> return
                }
            } catch (e: Exception) {
                Log.w(TAG, "event $kind: $payload", e)
                return
            }
            _events.tryEmit(ev)
        }
    }

    override fun openBrowser(url: String) {
        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        try {
            app.startActivity(intent)
        } catch (e: ActivityNotFoundException) {
            Log.w(TAG, "no browser for $url", e)
        }
    }

    override fun clearAuthUrl() {
        _authUrl.value = null
    }

    private suspend fun <T> io(block: () -> T): T = withContext(Dispatchers.IO) { block() }

    private inline fun <reified T> String.decode(): T = json.decodeFromString(this)

    // Settings

    override suspend fun settings(): Settings = io { Mobile.settings().decode() }

    /** Changes some settings fields, keeping the ones this app doesn't know. */
    override suspend fun updateSettings(changes: Map<String, JsonElement>): Settings = io {
        val current = json.parseToJsonElement(Mobile.settings()).jsonObject
        Mobile.saveSettings(JsonObject(current + changes).toString()).decode()
    }

    val logFile: String get() = runCatching { Mobile.logFile() }.getOrDefault("")

    // Cloud

    override suspend fun providers(): List<Provider> = io { Mobile.providers().decode() }
    override suspend fun remotes(): List<Remote> = io { Mobile.remotes().decode() }

    override suspend fun createRemote(name: String, type: String, params: Map<String, String>): Question = io {
        Mobile.createRemote(name, type, json.encodeToString(params)).decode()
    }

    override suspend fun continueRemote(name: String, state: String, answer: String): Question = io {
        Mobile.continueRemote(name, state, answer).decode()
    }

    override suspend fun cancelSignIn(): Boolean = io { Mobile.cancelSignIn() }

    override suspend fun deleteRemote(name: String) = io { Mobile.deleteRemote(name) }
    override suspend fun testRemote(remote: String, root: String) = io { Mobile.testRemote(remote, root) }

    // Library

    override suspend fun games(): List<GameView> = io { Mobile.games().decode() }
    override suspend fun game(id: String): GameView = io { Mobile.game(id).decode() }

    override suspend fun addGame(name: String, path: String): GameView = io {
        val input = NewGame(name, listOf(SaveSlot(key = "saves", label = "Saves")), NewDevice(mapOf("saves" to path)))
        Mobile.saveGame(json.encodeToString(input)).decode()
    }

    /** A game played with an emulator: its game file (path or content:// URI) and save folder here, both optional. */
    override suspend fun addEmulatorGame(name: String, emulatorId: String, rom: String, savePath: String): GameView = io {
        Mobile.addEmulatorGame(name, emulatorId, rom, savePath).decode()
    }

    override suspend fun renameGame(gameId: String, name: String): GameView = io { Mobile.renameGame(gameId, name).decode() }

    /** Adds an emulator to a game on every device; shareWith: an emulator of the game whose saves it uses, or "". */
    override suspend fun addGameEmulator(gameId: String, emulatorId: String, shareWith: String): GameView = io {
        Mobile.addGameEmulator(gameId, emulatorId, shareWith).decode()
    }

    override suspend fun removeGameEmulator(gameId: String, emulatorId: String): GameView = io {
        Mobile.removeGameEmulator(gameId, emulatorId).decode()
    }

    override suspend fun setSlots(key: String, slots: List<SaveSlot>): GameView = io {
        Mobile.setSlots(key, json.encodeToString(slots)).decode()
    }

    /** A game's name from its game file's name. */
    override suspend fun guessRomName(emulatorId: String, fileName: String): String = io { Mobile.guessRomName(emulatorId, fileName) }

    override suspend fun setPaths(key: String, paths: Map<String, String>): GameView = io {
        Mobile.setPaths(key, json.encodeToString(paths)).decode()
    }

    override suspend fun setRom(key: String, rom: String): GameView = io { Mobile.setRom(key, rom).decode() }
    override suspend fun setNativeApp(gameId: String, pkg: String): GameView = io { Mobile.setNativeApp(gameId, pkg).decode() }
    override suspend fun emulators(): List<EmulatorView> = io { Mobile.emulators().decode() }
    override suspend fun setCover(gameId: String, path: String): GameView = io { Mobile.setCover(gameId, path).decode() }
    /** Removes this device's setup of an emulator; the emulator, games and saves stay. */
    override suspend fun unsetEmulator(id: String) = io { Mobile.unsetEmulator(id) }

    override suspend fun saveEmulator(setup: EmulatorSetup): EmulatorView = io {
        Mobile.saveEmulator(json.encodeToString(setup)).decode()
    }

    override suspend fun previewPath(gameId: String, raw: String): String = io { Mobile.previewPath(gameId, raw) }
    override suspend fun checkLocalChanges() = io { Mobile.checkLocalChanges() }

    // Sync

    override suspend fun plan(key: String): PlanView = io { Mobile.plan(key).decode() }
    override suspend fun sync(key: String, resolution: Resolution?): SyncOutcome =
        io { Mobile.sync(key, resolution?.wire.orEmpty()).decode() }
    override suspend fun syncAll(): SyncAllOutcome = io { Mobile.syncAll().decode() }
    override suspend fun importRcloneConfig(contents: String): ImportResult = io { Mobile.importRcloneConfig(contents).decode() }
    override suspend fun suggestSaves(key: String, rom: String): SaveSuggestions = io { Mobile.suggestSaves(key, rom).decode() }
    override suspend fun useSaveSuggestion(key: String, suggestion: SaveSuggestion): GameView =
        io { Mobile.useSaveSuggestion(key, json.encodeToString(suggestion)).decode() }
    override suspend fun autofillSaves(key: String, rom: String): Int = io { Mobile.autofillSaves(key, rom).decode<AutofillResult>().applied }
    override suspend fun backups(key: String): List<Backup> = io { Mobile.backups(key).decode() }
    override suspend fun backupNow(key: String): SyncOutcome = io { Mobile.backupNow(key).decode() }
    override suspend fun setBackupPinned(key: String, name: String, pinned: Boolean) = io { Mobile.setBackupPinned(key, name, pinned) }
    override suspend fun deleteBackup(key: String, name: String) = io { Mobile.deleteBackup(key, name) }
    override suspend fun restoreBackup(key: String, name: String): SyncOutcome = io { Mobile.restoreBackup(key, name).decode() }
    override suspend fun syncLibrary() = io { Mobile.syncLibrary() }
    override fun cancel(key: String): Boolean = Mobile.cancel(key)
    override suspend fun waitBackground() = io { Mobile.waitBackground() }

    // Artwork from SteamGridDB

    override suspend fun testSteamGridKey(key: String) = io { Mobile.testSteamGridKey(key) }
    override suspend fun searchGridGames(term: String): List<GridGame> = io { Mobile.searchGridGames(term).decode() }

    override suspend fun gridArt(kind: ArtKind, gridGameId: Int): List<GridArt> =
        io { Mobile.gridArt(kind.wire, gridGameId.toLong()).decode() }

    override suspend fun setCoverFromUrl(gameId: String, url: String): GameView = io { Mobile.setCoverFromURL(gameId, url).decode() }

    override suspend fun setBackgroundFromUrl(gameId: String, url: String): GameView =
        io { Mobile.setBackgroundFromURL(gameId, url).decode() }

    override suspend fun clearBackground(gameId: String): GameView = io { Mobile.clearBackground(gameId).decode() }

    // Trash

    override suspend fun trash(): TrashView = io { Mobile.trash().decode() }
    override suspend fun deleteGame(id: String) = io { Mobile.deleteGame(id) }
    override suspend fun restoreGame(id: String) = io { Mobile.restoreGame(id) }
    override suspend fun deleteGameForever(id: String, withSaves: Boolean) = io { Mobile.deleteGameForever(id, withSaves) }
    override suspend fun deleteEmulator(id: String) = io { Mobile.deleteEmulator(id) }
    override suspend fun restoreEmulator(id: String) = io { Mobile.restoreEmulator(id) }
    override suspend fun deleteEmulatorForever(id: String, withSaves: Boolean) = io { Mobile.deleteEmulatorForever(id, withSaves) }
    override suspend fun leftovers(): List<Leftover> = io { Mobile.leftovers().decode() }
    override suspend fun restoreLeftover(key: String): String = io { Mobile.restoreLeftover(key) }
    override suspend fun deleteLeftover(key: String) = io { Mobile.deleteLeftover(key) }

    // Play sessions (the game is started by the app; see SessionService)

    /** Syncs before playing and takes the cloud lock; prompts arrive as events meanwhile. */
    override suspend fun beginSession(key: String, emulator: String): BeginOutcome = io {
        Mobile.beginSession(key, emulator).decode()
    }

    /** Uploads after playing (never downloads) and releases the cloud lock. */
    override suspend fun endSession(key: String): EndOutcome = io { Mobile.endSession(key).decode() }
    override suspend fun openSessions(): List<OpenSession> = io { Mobile.openSessions().decode() }

    /** Sessions whose game closed but whose saves weren't uploaded (offline then). */
    override suspend fun pendingSessions(): List<OpenSession> = io { Mobile.pendingSessions().decode() }

    /** Finishes a pending session; does nothing if it was finished or the game was started again. */
    override suspend fun endPendingSession(key: String): EndOutcome = io { Mobile.endPendingSession(key).decode() }
    override fun sessionOpen(key: String): Boolean = Mobile.sessionOpen(key)
    override fun respond(promptId: String, answer: String): Boolean = Mobile.respond(promptId, answer)
    override suspend fun prompts(): List<Prompt> = io { Mobile.prompts().decode() }
}
