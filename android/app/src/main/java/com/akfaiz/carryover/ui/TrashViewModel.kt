package com.akfaiz.carryover.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Leftover
import com.akfaiz.carryover.core.TrashEmulator
import com.akfaiz.carryover.core.TrashGame
import com.akfaiz.carryover.data.AppServices
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the trash page shows. */
data class TrashState(
    val loaded: Boolean = false,
    /** How long a deleted game or emulator stays restorable. */
    val days: Int = 30,
    val games: List<TrashGame> = emptyList(),
    val emulators: List<TrashEmulator> = emptyList(),
    /** The cloud save folders no game uses; null until the user looked in the cloud. */
    val leftovers: List<Leftover>? = null,
    val lookingInCloud: Boolean = false,
    /** Ids and keys with an action running. */
    val busy: Set<String> = emptySet(),
) {
    val isEmpty: Boolean get() = games.isEmpty() && emulators.isEmpty()
}

/**
 * The trash page: deleted games and emulators (restore, or delete for good with
 * or without their cloud saves) and the cloud save folders no game uses (bring
 * them back, or delete them). Deleting a game or an emulator in the first place
 * is [AppViewModel.deleteGame] / [AppViewModel.deleteEmulator].
 */
class TrashViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {
    private val core get() = services.core
    private val m get() = services.messenger

    private val _state = MutableStateFlow(TrashState())
    val state: StateFlow<TrashState> = _state

    init {
        if (core.initError == null) refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            val t = m.attempt(m.str(R.string.err_read_trash)) { core.trash() } ?: return@launch
            _state.update { it.copy(loaded = true, days = t.days, games = t.games, emulators = t.emulators) }
        }
    }

    fun restoreGame(g: TrashGame) = act(g.id, m.str(R.string.err_restore, g.name)) {
        core.restoreGame(g.id)
        m.say(m.str(R.string.msg_restored, g.name))
    }

    fun restoreEmulator(e: TrashEmulator) = act(e.id, m.str(R.string.err_restore, e.name)) {
        core.restoreEmulator(e.id)
        m.say(m.str(R.string.msg_restored, e.name))
    }

    /** Deletes a game for good; [withSaves] also deletes what it has in the cloud (which may take a while). */
    fun deleteGameForever(g: TrashGame, withSaves: Boolean) = act(g.id, m.str(R.string.err_delete_game, g.name)) {
        core.deleteGameForever(g.id, withSaves)
        m.say(m.str(R.string.msg_deleted_forever, g.name))
    }

    fun deleteEmulatorForever(e: TrashEmulator, withSaves: Boolean) = act(e.id, m.str(R.string.err_delete_game, e.name)) {
        core.deleteEmulatorForever(e.id, withSaves)
        m.say(m.str(R.string.msg_deleted_forever, e.name))
    }

    /** Looks in the cloud for save folders no game uses (one cloud call, so only when asked). */
    fun lookInCloud() {
        if (_state.value.lookingInCloud) return
        _state.update { it.copy(lookingInCloud = true) }
        viewModelScope.launch {
            try {
                val found = m.attempt(m.str(R.string.err_leftovers)) { core.leftovers() }
                if (found != null) _state.update { it.copy(leftovers = found) }
            } finally {
                _state.update { it.copy(lookingInCloud = false) }
            }
        }
    }

    /** Brings a leftover back as a game: it still needs its save folder on this device. */
    fun restoreLeftover(l: Leftover) = act(l.key, m.str(R.string.err_leftover_restore, l.title)) {
        core.restoreLeftover(l.key)
        m.say(m.str(R.string.msg_leftover_restored, l.title))
    }

    fun deleteLeftover(l: Leftover) = act(l.key, m.str(R.string.err_leftover_delete, l.title)) {
        core.deleteLeftover(l.key)
        m.say(m.str(R.string.msg_leftover_deleted, l.title))
    }

    /**
     * Runs [work] for [id] (once at a time), says it failed as [what] if it
     * does, then reads the trash again, and the leftovers if they were looked at
     * (deleting a game for good without its saves makes one).
     */
    private fun act(id: String, what: String, work: suspend () -> Unit) {
        if (id in _state.value.busy) return
        _state.update { it.copy(busy = it.busy + id) }
        viewModelScope.launch {
            try {
                m.attempt(what) { work() }
                val t = m.attempt(m.str(R.string.err_read_trash)) { core.trash() }
                if (t != null) _state.update { it.copy(loaded = true, days = t.days, games = t.games, emulators = t.emulators) }
                if (_state.value.leftovers != null) {
                    val found = m.attempt(m.str(R.string.err_leftovers)) { core.leftovers() }
                    if (found != null) _state.update { it.copy(leftovers = found) }
                }
            } finally {
                _state.update { it.copy(busy = it.busy - id) }
            }
        }
    }
}
