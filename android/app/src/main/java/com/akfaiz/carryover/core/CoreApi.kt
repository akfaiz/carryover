package com.akfaiz.carryover.core

import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.serialization.json.JsonElement

/**
 * What the session code (the watcher, the retry worker) needs of the Go core.
 * Its own interface so that logic can run against a fake in unit tests: the
 * real [Core] loads the gomobile library, which a JVM test can't.
 */
interface SessionCore {
    /** Syncs before playing and takes the cloud lock; prompts arrive as events meanwhile. */
    suspend fun beginSession(key: String, emulator: String = ""): BeginOutcome

    /** Uploads after playing (never downloads) and releases the cloud lock. */
    suspend fun endSession(key: String): EndOutcome
    suspend fun openSessions(): List<OpenSession>

    /** Sessions whose game closed but whose saves weren't uploaded (offline then). */
    suspend fun pendingSessions(): List<OpenSession>

    /** Finishes a pending session; does nothing if it was finished or the game was started again. */
    suspend fun endPendingSession(key: String): EndOutcome
    fun sessionOpen(key: String): Boolean
    suspend fun games(): List<GameView>
    suspend fun emulators(): List<EmulatorView>
}

/** What the settings and cloud pages need of the Go core (see [com.akfaiz.carryover.data.SettingsRepository]). */
interface SettingsCore {
    suspend fun settings(): Settings

    /** Changes some settings fields, keeping the ones this app doesn't know. */
    suspend fun updateSettings(changes: Map<String, JsonElement>): Settings
    suspend fun remotes(): List<Remote>
    suspend fun deleteRemote(name: String)
    suspend fun testRemote(remote: String, root: String)
    suspend fun syncLibrary()

    /** Checks a SteamGridDB API key without saving it; throws when SteamGridDB rejects it. */
    suspend fun testSteamGridKey(key: String)
}

/** What syncing a game (or all of them) needs of the Go core (see [com.akfaiz.carryover.data.SyncController]). */
interface SyncCore {
    /** What a sync would do (for a conflict: what each side has). */
    suspend fun plan(key: String): PlanView

    /** Syncs profile [key]; [resolution] decides a conflict. */
    suspend fun sync(key: String, resolution: Resolution? = null): SyncOutcome
    suspend fun syncAll(): SyncAllOutcome

    /** Stops the running sync of [key] ("" stops Sync all); whether one was running. */
    fun cancel(key: String): Boolean
}

/**
 * Where an emulator keeps a game's saves, from the emulator's manifest and its data folder on this device (see
 * [com.akfaiz.carryover.ui.SuggestViewModel]).
 */
interface SuggestCore {
    /**
     * The save locations of the emulator profile [key]. [rom] is the game file's name or path ("" = the profile's own).
     * Needs the emulator's type and data folder; downloads the manifest the first time.
     */
    suspend fun suggestSaves(key: String, rom: String = ""): SaveSuggestions

    /** Makes [suggestion] this device's save folder of profile [key]. */
    suspend fun useSaveSuggestion(key: String, suggestion: SaveSuggestion): GameView

    /** Fills in the save folders of a profile that has none, with what is on this device; how many were filled in. */
    suspend fun autofillSaves(key: String, rom: String = ""): Int
}

/**
 * The backups of a game's saves, per profile key (see [com.akfaiz.carryover.ui.BackupsViewModel]); they live in the
 * cloud, next to the saves, except the copies a device keeps for itself.
 */
interface BackupCore {
    /** The backups of profile [key], newest first. Needs the cloud. */
    suspend fun backups(key: String): List<Backup>

    /** Stores a pinned backup of this device's saves, whether or not automatic backups are on. */
    suspend fun backupNow(key: String): SyncOutcome
    suspend fun setBackupPinned(key: String, name: String, pinned: Boolean)

    /** Deletes a backup for good, for every device. */
    suspend fun deleteBackup(key: String, name: String)

    /** Replaces the saves on this device, then in the cloud, with a backup (this device's are backed up first). */
    suspend fun restoreBackup(key: String, name: String): SyncOutcome
}

/**
 * Deleting games and emulators: the trash (restorable for [TrashView.days]), deleting for good with or without
 * the cloud saves, and the cloud save folders no game uses (see [com.akfaiz.carryover.ui.TrashViewModel]).
 */
interface TrashCore {
    /** What can still be restored; first removes what has been in the trash too long. */
    suspend fun trash(): TrashView

    /** Moves a game to the trash on all devices; its cloud saves stay. */
    suspend fun deleteGame(id: String)
    suspend fun restoreGame(id: String)

    /** Deletes a game, live or in the trash, for good; [withSaves] also deletes everything it has in the cloud. */
    suspend fun deleteGameForever(id: String, withSaves: Boolean)
    suspend fun deleteEmulator(id: String)
    suspend fun restoreEmulator(id: String)
    suspend fun deleteEmulatorForever(id: String, withSaves: Boolean)

    /** The cloud save folders no game uses. Needs the cloud, and takes a moment. */
    suspend fun leftovers(): List<Leftover>

    /** Adds a leftover cloud folder back to the library; the game's id. */
    suspend fun restoreLeftover(key: String): String
    suspend fun deleteLeftover(key: String)
}

/** Finding covers and backgrounds on SteamGridDB (needs the API key in the settings). */
interface ArtCore {
    suspend fun searchGridGames(term: String): List<GridGame>

    /** Artwork of a SteamGridDB game, best scored first. */
    suspend fun gridArt(kind: ArtKind, gridGameId: Int): List<GridArt>

    /** Downloads an image of [searchGridGames] / [gridArt] as the game's cover. */
    suspend fun setCoverFromUrl(gameId: String, url: String): GameView

    /** Downloads an image as the background of the game's page. */
    suspend fun setBackgroundFromUrl(gameId: String, url: String): GameView
    suspend fun clearBackground(gameId: String): GameView
}

/**
 * The whole bridge to the Go core (package mobile): JSON in and out, decoded
 * into the types of Models.kt. Calls suspend and run off the main thread; Go
 * errors arrive as exceptions whose message is the error. The view models take
 * this, not the [Core] object.
 */
interface CoreApi : SessionCore, SettingsCore, SyncCore, BackupCore, SuggestCore, TrashCore, ArtCore {
    /** Why the core couldn't start, or null when it runs. */
    val initError: String?
    val events: SharedFlow<CoreEvent>

    /** The authorisation page of the cloud provider being connected, if any. */
    val authUrl: StateFlow<String?>
    val version: String
    fun openBrowser(url: String)
    fun clearAuthUrl()

    // Cloud
    suspend fun providers(): List<Provider>
    suspend fun createRemote(name: String, type: String, params: Map<String, String>): Question
    suspend fun continueRemote(name: String, state: String, answer: String): Question

    /** Copies the accounts of supported kinds from the contents of an rclone.conf; an account of the same name is replaced. */
    suspend fun importRcloneConfig(contents: String): ImportResult

    // Library
    suspend fun game(id: String): GameView
    suspend fun addGame(name: String, path: String): GameView

    /** A game played with an emulator: its game file (path or content:// URI) and save folder here, both optional. */
    suspend fun addEmulatorGame(name: String, emulatorId: String, rom: String, savePath: String): GameView
    suspend fun renameGame(gameId: String, name: String): GameView

    /** Adds an emulator to a game on every device; shareWith: an emulator of the game whose saves it uses, or "". */
    suspend fun addGameEmulator(gameId: String, emulatorId: String, shareWith: String): GameView
    suspend fun removeGameEmulator(gameId: String, emulatorId: String): GameView

    /** A game's name from its game file's name. */
    suspend fun guessRomName(emulatorId: String, fileName: String): String
    suspend fun setPaths(key: String, paths: Map<String, String>): GameView
    suspend fun setRom(key: String, rom: String): GameView
    suspend fun setNativeApp(gameId: String, pkg: String): GameView
    suspend fun setCover(gameId: String, path: String): GameView

    /** Removes this device's setup of an emulator; the emulator, games and saves stay. */
    suspend fun unsetEmulator(id: String)
    suspend fun saveEmulator(setup: EmulatorSetup): EmulatorView

    suspend fun previewPath(gameId: String, raw: String): String
    suspend fun checkLocalChanges()

    // Sync
    suspend fun waitBackground()

    // Questions of play sessions
    fun respond(promptId: String, answer: String): Boolean
    suspend fun prompts(): List<Prompt>
}
