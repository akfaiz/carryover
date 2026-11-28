package com.akfaiz.carryover.ui.backups

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Backup
import com.akfaiz.carryover.core.SyncOutcome
import com.akfaiz.carryover.data.AppServices
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the backups page shows. */
data class BackupsState(
    /** The profile the page is about (a game id, or `<game>.<emulator>`). */
    val key: String = "",
    val loaded: Boolean = false,
    /** The list could not be read (no cloud, or it can't be reached). */
    val loadFailed: Boolean = false,
    val backups: List<Backup> = emptyList(),
    /** "Back up now" is running. */
    val making: Boolean = false,
    /** The name of the backup an action (pin, delete, restore) is running for. */
    val acting: String? = null,
    /** The acting one is a restore (the only action of a backup that can be cancelled). */
    val restoring: Boolean = false,
    /** How many cloud backups are kept per game; 0 = cloud backups are off. */
    val kept: Int = 0,
) {
    /** One action at a time: they all touch the same save folder and cloud folder. */
    val busy: Boolean get() = making || acting != null

    /** Whether Cancel does something: backing up and restoring can be stopped. */
    val cancellable: Boolean get() = making || restoring
}

/**
 * The backups of one profile: list them, make one now, pin or unpin, delete,
 * and restore (which replaces the saves on this device and in the cloud). The
 * rules (what is kept, what a restore backs up first) are the core's; restoring
 * changes the game's status, which reaches the library through the core's
 * library event.
 */
class BackupsViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {
    private val core get() = services.core
    private val m get() = services.messenger

    private val _state = MutableStateFlow(BackupsState())
    val state: StateFlow<BackupsState> = _state

    init {
        viewModelScope.launch {
            services.settings.state.map { it.backupsKept }.distinctUntilChanged().collect { k -> _state.update { it.copy(kept = k) } }
        }
    }

    /** Bumped when the page is about another profile, so a late list of the previous one is dropped. */
    private var generation = 0

    /** Opens the page for [key] (a profile of a game): the list of another one is cleared at once. */
    fun open(key: String) {
        if (key != _state.value.key) {
            generation++
            _state.value = BackupsState(key = key, kept = _state.value.kept)
        }
        refresh()
    }

    fun refresh() {
        val key = _state.value.key
        if (key.isEmpty() || core.initError != null) return
        val mine = generation
        viewModelScope.launch {
            val list = m.attempt(m.str(R.string.err_read_backups)) { core.backups(key) }
            if (mine != generation) return@launch
            _state.update { it.copy(loaded = true, loadFailed = list == null, backups = list ?: it.backups) }
        }
    }

    /** Stores a pinned backup of this device's saves. */
    fun backupNow() {
        val s = _state.value
        if (s.busy) return
        _state.update { it.copy(making = true) }
        run(m.str(R.string.err_backup_now), onDone = { _state.update { it.copy(making = false) } }) {
            report(core.backupNow(s.key), R.string.msg_backup_made)
        }
    }

    /** Pins a backup, or unpins a pinned one. */
    fun togglePinned(b: Backup) = act(b, m.str(R.string.err_backup_change)) {
        core.setBackupPinned(_state.value.key, b.name, !b.pinned)
    }

    fun delete(b: Backup) = act(b, m.str(R.string.err_backup_delete)) {
        core.deleteBackup(_state.value.key, b.name)
        m.say(m.str(R.string.msg_backup_deleted))
    }

    /** Replaces the saves on this device and in the cloud with [b]. */
    fun restore(b: Backup) = act(b, m.str(R.string.err_backup_restore), restoring = true) {
        report(core.restoreBackup(_state.value.key, b.name), R.string.msg_backup_restored)
    }

    /** Stops the running backup or restore (what was already transferred is kept or cleaned up by the core). */
    fun cancel() {
        core.cancel(_state.value.key)
    }

    /** Tells how a cancellable action ended; being cancelled is not worth a message. */
    private fun report(o: SyncOutcome, done: Int) {
        when {
            o.cancelled -> Unit
            o.offline -> m.say(m.str(R.string.msg_backup_offline))
            else -> m.say(m.str(done))
        }
    }

    private fun act(b: Backup, what: String, restoring: Boolean = false, work: suspend () -> Unit) {
        if (_state.value.busy) return
        _state.update { it.copy(acting = b.name, restoring = restoring) }
        run(what, onDone = { _state.update { it.copy(acting = null, restoring = false) } }, work = work)
    }

    /** Runs [work] and says it failed as [what] if it does; then lists the backups again and calls [onDone]. */
    private fun run(what: String, onDone: () -> Unit, work: suspend () -> Unit) {
        viewModelScope.launch {
            try {
                m.attempt(what) { work() }
                refresh()
            } finally {
                onDone()
            }
        }
    }
}
