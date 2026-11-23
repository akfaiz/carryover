package com.akfaiz.carryover.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.data.AppServices
import com.akfaiz.carryover.core.DeviceView
import com.akfaiz.carryover.data.SettingsState
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.File

/**
 * The settings page: theme, play-session preferences, the device name and the
 * logs for a bug report. The state is the [com.akfaiz.carryover.data.SettingsRepository]'s,
 * shared with the pages that read the theme or whether a cloud is set up.
 */
class SettingsViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {

    private val repo get() = services.settings
    private val m get() = services.messenger

    val state: StateFlow<SettingsState> get() = repo.state

    /** The core's version, for the About section. */
    val coreVersion: String get() = services.core.version

    /** Sets the theme ("", "light", "dark"); kept in the core's settings like the desktop's. */
    fun setTheme(theme: String) {
        viewModelScope.launch { m.attempt(m.str(R.string.err_save)) { repo.setTheme(theme) } }
    }

    fun setWallpaper(on: Boolean) = repo.setWallpaper(on)

    /**
     * Saves the SteamGridDB API key (an empty one removes it). SteamGridDB
     * checks it first: a key it rejects is said and not saved; [done] says
     * whether it was saved.
     */
    fun saveSteamGridKey(key: String, done: (Boolean) -> Unit) {
        viewModelScope.launch {
            val ok = m.attempt(m.str(R.string.err_grid_key)) {
                repo.setSteamGridKey(key)
                m.say(m.str(if (key.isBlank()) R.string.msg_grid_key_removed else R.string.msg_grid_key_saved))
            }
            done(ok != null)
        }
    }

    /** Shares the library through the cloud (and merges it now), or keeps it on this device only. */
    fun setShareLibrary(on: Boolean) = changeLibrarySetting(merge = on) { repo.setShareLibrary(on) }

    /** Syncs covers and artwork with the library (and fetches them now), or keeps them on this device only. */
    fun setSyncCovers(on: Boolean) = changeLibrarySetting(merge = on) { repo.setSyncCovers(on) }

    /**
     * Saves a library setting, then (when [merge]) merges the library with the cloud so what it now covers shows up.
     * A merge that fails does not undo the setting: it is said, and the next sync does it again.
     */
    private fun changeLibrarySetting(merge: Boolean, save: suspend () -> Unit) {
        viewModelScope.launch {
            if (m.attempt(m.str(R.string.err_library_setting)) { save() } == null) return@launch
            if (merge) m.attempt(m.str(R.string.err_library_refresh)) { repo.refreshLibrary() }
        }
    }

    /** Keeps the last [keep] cloud backups per game (0 = off). */
    fun setBackupsKept(keep: Int) {
        viewModelScope.launch {
            m.attempt(m.str(R.string.err_backups_setting)) {
                repo.setBackupsKept(keep)
                m.say(if (keep == 0) m.str(R.string.msg_backups_off) else m.plural(R.plurals.msg_backups_on, keep, keep))
            }
        }
    }

    /** Opens a page (where to get a SteamGridDB key) in the browser. */
    fun openBrowser(url: String) = services.core.openBrowser(url)

    /** Seconds a game may be out of front before its play session ends. */
    fun setGraceSeconds(seconds: Int) = repo.setGraceSeconds(seconds)

    /** Turns the periodic background sync on or off (WorkManager). */
    fun setBackgroundSync(on: Boolean) = repo.setBackgroundSync(on)

    fun setDeviceName(name: String) {
        viewModelScope.launch {
            m.attempt(m.str(R.string.err_save)) {
                repo.setDeviceName(name)
                m.say(m.str(R.string.msg_saved))
            }
        }
    }

    // Other devices

    private val _devices = MutableStateFlow<List<DeviceView>>(emptyList())

    /** The devices the library holds settings of, this one first; read by [loadDevices]. */
    val devices: StateFlow<List<DeviceView>> = _devices.asStateFlow()

    /** Reads the devices the library knows (again). */
    fun loadDevices() {
        viewModelScope.launch { m.attempt(m.str(R.string.err_devices)) { _devices.value = repo.devices() } }
    }

    /** Removes [device] (another one, no longer used) from the library on every device. */
    fun forgetDevice(device: DeviceView) = changeDevice(m.str(R.string.msg_device_forgotten)) { repo.forgetDevice(device.id) }

    /** Makes this device [device] again: an earlier install of the app here, whose settings it takes back. */
    fun reuseDevice(device: DeviceView, name: String) =
        changeDevice(m.str(R.string.msg_device_reused, name)) { repo.reuseDevice(device.id) }

    private fun changeDevice(done: String, change: suspend () -> Unit) {
        viewModelScope.launch {
            if (m.attempt(m.str(R.string.err_device_change)) { change() } != null) m.say(done)
            m.attempt(m.str(R.string.err_devices)) { _devices.value = repo.devices() }
        }
    }

    // Logs for a bug report

    /** Collects the logs into a zip (BugReport); null, with a message, when that fails. */
    suspend fun collectLogs(): File? = m.attempt(m.str(R.string.err_logs)) { services.platform.buildLogs() }

    /** Saves a zip from collectLogs to the document the user picked. */
    fun saveLogs(zip: File, target: String) {
        viewModelScope.launch {
            try {
                services.platform.saveLogs(zip, target)
                m.say(m.str(R.string.msg_logs_saved))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                m.failed(m.str(R.string.err_logs), e)
            }
        }
    }
}
