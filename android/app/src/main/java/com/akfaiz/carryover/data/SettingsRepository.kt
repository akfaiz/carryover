package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.DeviceView
import com.akfaiz.carryover.core.Remote
import com.akfaiz.carryover.core.Settings
import com.akfaiz.carryover.core.SettingsCore
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.serialization.json.JsonPrimitive

/** The app's own preferences, read before the Go core is open (AppPrefs on Android). */
interface PreferenceStore {
    /** The grace periods to choose from, in seconds. */
    val graceChoices: List<Int>

    /** "" (follow the system), "light" or "dark": a copy of the core's Settings.Theme for the first frame. */
    var theme: String

    /** Use the wallpaper's colours (Android 12+) instead of Carryover's. */
    var wallpaper: Boolean

    /** Seconds a game may be out of front before its play session ends. */
    var graceSeconds: Int

    /** Sync all games now and then while the device is charging, idle and on an unmetered network. */
    var backgroundSync: Boolean
}

/** The most cloud backups per game the core accepts. */
const val MAX_BACKUPS = 100

/** Settings and cloud accounts as the pages show them. */
data class SettingsState(
    /** The core's settings (shared with the desktop app's); empty until [SettingsRepository.refresh] ran. */
    val settings: Settings = Settings(),
    val remotes: List<Remote> = emptyList(),
    val theme: String = "",
    val wallpaper: Boolean = false,
    val graceSeconds: Int = 10,
    val graceChoices: List<Int> = listOf(10, 30, 60),
    val backgroundSync: Boolean = false,
) {
    val cloudReady: Boolean get() = settings.remote.isNotEmpty()

    /** A SteamGridDB API key is set: covers and backgrounds can be looked up. */
    val hasGridKey: Boolean get() = settings.steamGridDbKey.isNotEmpty()

    /** How many cloud backups are kept per game; 0 = cloud backups are off. */
    val backupsKept: Int get() = settings.backupRetention.coerceAtLeast(0)

    /** The library is shared with the other devices through the cloud. */
    val sharesLibrary: Boolean get() = settings.syncLibrary

    /** The releases updates come from: "" (finished ones only) or "prerelease". */
    val updateChannel: String get() = settings.updateChannel

    /** Newer versions are looked for without being asked. */
    val checksUpdates: Boolean get() = !settings.noUpdateCheck

    /** Covers and artwork go to the cloud and come from it (only meaningful while the library is shared). */
    val syncsCovers: Boolean get() = !settings.coversLocalOnly
}

/**
 * The settings of the app: the core's (theme, device name, the cloud) and the
 * app's own preferences, in one state that the settings, cloud and library
 * pages and the theme read. The functions throw what the core throws, and
 * their callers tell the user; no Android classes, so it is unit tested with a
 * fake [SettingsCore].
 */
class SettingsRepository(
    private val core: SettingsCore,
    private val prefs: PreferenceStore,
    /** Applies the background-sync preference (schedules or cancels the periodic work). */
    private val applyBackgroundSync: () -> Unit,
) {
    private val _state = MutableStateFlow(
        SettingsState(
            theme = prefs.theme,
            wallpaper = prefs.wallpaper,
            graceSeconds = prefs.graceSeconds,
            graceChoices = prefs.graceChoices,
            backgroundSync = prefs.backgroundSync,
        ),
    )
    val state: StateFlow<SettingsState> = _state

    private val _libraryStale = MutableSharedFlow<Unit>(extraBufferCapacity = 1)

    /** The library came from another place (a cloud was connected): reload it. */
    val libraryStale: SharedFlow<Unit> = _libraryStale

    /** Reads the settings and the accounts from the core. */
    suspend fun refresh() {
        val settings = core.settings()
        val remotes = core.remotes()
        // The core's setting wins (it is shared with the desktop app's); the
        // cached copy only serves the first frame.
        prefs.theme = settings.theme
        _state.update { it.copy(settings = settings, remotes = remotes, theme = settings.theme) }
    }

    /**
     * Sets the theme at once (the screen follows without waiting for the
     * core), then keeps it in the core's settings like the desktop's. A core
     * that fails leaves the choice in effect here.
     */
    suspend fun setTheme(theme: String) {
        prefs.theme = theme
        _state.update { it.copy(theme = theme) }
        val st = core.updateSettings(mapOf("theme" to JsonPrimitive(theme)))
        _state.update { it.copy(settings = st) }
    }

    fun setWallpaper(on: Boolean) {
        prefs.wallpaper = on
        _state.update { it.copy(wallpaper = on) }
    }

    fun setGraceSeconds(seconds: Int) {
        prefs.graceSeconds = seconds
        _state.update { it.copy(graceSeconds = seconds) }
    }

    fun setBackgroundSync(on: Boolean) {
        prefs.backgroundSync = on
        applyBackgroundSync()
        _state.update { it.copy(backgroundSync = on) }
    }

    suspend fun setDeviceName(name: String) {
        val st = core.updateSettings(mapOf("deviceName" to JsonPrimitive(name.trim())))
        _state.update { it.copy(settings = st) }
    }

    /**
     * Makes [remote]:[root] the cloud: checks it can be used, saves it, and
     * syncs the library from it (which is then stale, see [libraryStale]).
     */
    suspend fun useCloud(remote: String, root: String) {
        core.testRemote(remote, root)
        val st = core.updateSettings(
            mapOf(
                "remote" to JsonPrimitive(remote),
                "remoteRoot" to JsonPrimitive(root),
                "syncLibrary" to JsonPrimitive(true),
            ),
        )
        _state.update { it.copy(settings = st) }
        refreshLibrary()
    }

    /** Merges the library (and its images) with the cloud; the pages read the library again after it. */
    suspend fun refreshLibrary() {
        core.syncLibrary()
        _libraryStale.tryEmit(Unit)
    }

    /** The devices the library holds settings of: this one first. */
    suspend fun devices(): List<DeviceView> = core.devices()

    /** Removes another device's settings from the library; the pages read the library again after it. */
    suspend fun forgetDevice(id: String) {
        core.forgetDevice(id)
        _libraryStale.tryEmit(Unit)
    }

    /** Makes this device the earlier device [id] again: its id changes, and the library with it. */
    suspend fun reuseDevice(id: String) {
        core.reuseDevice(id)
        refresh()
        _libraryStale.tryEmit(Unit)
    }

    /** Shares the library between devices through the cloud, or keeps it on this device only. */
    suspend fun setShareLibrary(on: Boolean) {
        val st = core.updateSettings(mapOf("syncLibrary" to JsonPrimitive(on)))
        _state.update { it.copy(settings = st) }
    }

    /** Syncs covers and artwork with the library, or keeps them on this device only. */
    suspend fun setSyncCovers(on: Boolean) {
        val st = core.updateSettings(mapOf("coversLocalOnly" to JsonPrimitive(!on)))
        _state.update { it.copy(settings = st) }
    }

    /**
     * Saves the SteamGridDB API key. A key SteamGridDB rejects is not saved
     * (the error is thrown); an empty one removes the key.
     */
    suspend fun setSteamGridKey(key: String) {
        val k = key.trim()
        if (k.isNotEmpty()) core.testSteamGridKey(k)
        val st = core.updateSettings(mapOf("steamGridDbKey" to JsonPrimitive(k)))
        _state.update { it.copy(settings = st) }
    }

    /** Keeps the last [keep] cloud backups per game (1 to 100), or none with 0 (the core's setting, shared with the desktop app). */
    suspend fun setBackupsKept(keep: Int) {
        require(keep in 0..MAX_BACKUPS) { "backups to keep must be a number from 0 to $MAX_BACKUPS" }
        val st = core.updateSettings(mapOf("backupRetention" to JsonPrimitive(keep)))
        _state.update { it.copy(settings = st) }
    }

    /** The releases updates come from: "" (finished ones only) or "prerelease" (the core's setting). */
    suspend fun setUpdateChannel(channel: String) {
        val st = core.updateSettings(mapOf("updateChannel" to JsonPrimitive(channel)))
        _state.update { it.copy(settings = st) }
    }

    /** Looks for newer versions without being asked, or not. */
    suspend fun setCheckUpdates(on: Boolean) {
        val st = core.updateSettings(mapOf("noUpdateCheck" to JsonPrimitive(!on)))
        _state.update { it.copy(settings = st) }
    }

    suspend fun deleteRemote(name: String) {
        core.deleteRemote(name)
    }
}
