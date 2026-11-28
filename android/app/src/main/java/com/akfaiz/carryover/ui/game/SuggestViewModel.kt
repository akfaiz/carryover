package com.akfaiz.carryover.ui.game

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.SaveSuggestion
import com.akfaiz.carryover.core.SaveSuggestions
import com.akfaiz.carryover.data.AppServices
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the suggested save folders dialog shows; [key] is null while it is closed. */
data class SuggestState(
    /** The emulator profile the dialog is about. */
    val key: String? = null,
    val loading: Boolean = false,
    val result: SaveSuggestions? = null,
    /** Why there is no result (no data folder yet, the manifest couldn't be loaded): the core's own words. */
    val error: String? = null,
    /** A suggestion is being saved. */
    val applying: Boolean = false,
)

/**
 * Suggested save folders of a game's emulator profile: where the emulator keeps
 * this game's saves, from its manifest and its data folder on this device. The
 * user chooses one (it becomes the profile's save folder, with the filters the
 * manifest gives); the game then reaches the library through the core's library
 * event.
 */
class SuggestViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {
    private val core get() = services.core
    private val m get() = services.messenger

    private val _state = MutableStateFlow(SuggestState())
    val state: StateFlow<SuggestState> = _state

    /** Bumped when the dialog opens or closes, so the late answer of a lookup nobody waits for is dropped. */
    private var generation = 0

    /** Opens the dialog for profile [key] and looks the locations up. */
    fun open(key: String) {
        val mine = ++generation
        _state.value = SuggestState(key = key, loading = true)
        viewModelScope.launch {
            val found = try {
                core.suggestSaves(key) to null
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                null to m.error(e).replaceFirstChar { it.uppercase() }
            }
            if (mine != generation) return@launch
            _state.update { it.copy(loading = false, result = found.first, error = found.second) }
        }
    }

    fun close() {
        generation++
        _state.value = SuggestState()
    }

    /** Makes [suggestion] the profile's save folder and closes the dialog. */
    fun use(suggestion: SaveSuggestion) {
        val s = _state.value
        val key = s.key ?: return
        if (s.applying) return
        _state.update { it.copy(applying = true) }
        viewModelScope.launch {
            val ok = m.attempt(m.str(R.string.err_suggest_use)) { core.useSaveSuggestion(key, suggestion) }
            _state.update { it.copy(applying = false) }
            if (ok != null) {
                m.say(m.str(R.string.msg_suggest_used))
                if (_state.value.key == key) close()
            }
        }
    }
}
