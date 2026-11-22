package com.akfaiz.carryover.launch

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.CoreApi
import com.akfaiz.carryover.core.CoreEvent
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.Prompt
import com.akfaiz.carryover.data.Texts
import com.akfaiz.carryover.session.PlayOutcome
import com.akfaiz.carryover.session.Sessions
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the launch screen shows. */
sealed interface LaunchState {
    data class Working(val message: String) : LaunchState

    /** The launch's file is in no game of Carryover: which one is it? */
    data class Choose(val app: String, val file: String, val candidates: List<Forward.Candidate>) : LaunchState
    data class Error(val message: String) : LaunchState
    data object Done : LaunchState
}

/**
 * A launch from a frontend or a shortcut (LaunchActivity): find the game
 * profile ([LaunchPlanner]), play it as the Play button does, start the game,
 * close. Made for one launch: [platform] holds the intent that started it.
 */
class LaunchViewModel(
    private val core: CoreApi,
    private val platform: LaunchPlatform,
    private val texts: Texts,
) : ViewModel() {
    private val _state = MutableStateFlow<LaunchState>(LaunchState.Working(texts.string(R.string.status_checking)))
    val state: StateFlow<LaunchState> = _state
    private val _prompts = MutableStateFlow<List<Prompt>>(emptyList())
    val prompts: StateFlow<List<Prompt>> = _prompts

    /** A frontend's launch is prepared to be started ([LaunchPlatform.prepareForward]). */
    private var forwardReady = false
    private var forwardPkg = ""
    private var rom: String? = null
    private var started = false

    init {
        viewModelScope.launch {
            core.events.collect { ev ->
                when (ev) {
                    is CoreEvent.PromptOpened -> _prompts.update { it + ev.prompt }
                    is CoreEvent.PromptClosed -> _prompts.update { l -> l.filter { it.id != ev.id } }
                    is CoreEvent.Status -> _state.update { s ->
                        if (s is LaunchState.Working && ev.status.message.isNotEmpty()) LaunchState.Working(ev.status.message) else s
                    }
                    else -> {}
                }
            }
        }
    }

    fun answer(p: Prompt, answer: String) {
        core.respond(p.id, answer)
        _prompts.update { l -> l.filter { it.id != p.id } }
    }

    private fun str(id: Int, vararg args: Any): String = texts.string(id, *args)

    private fun fail(msg: String) {
        _state.value = LaunchState.Error(msg)
    }

    /** Handles the launch (once). */
    fun start() {
        if (started) return
        started = true
        core.initError?.let { return fail(str(R.string.launch_init_failed, it)) }
        viewModelScope.launch {
            try {
                handle()
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                platform.log("launch", e)
                fail(e.message ?: e.toString())
            }
        }
    }

    private suspend fun handle() {
        val games = core.games()
        val emulators = core.emulators()
        val r = platform.request
        when (val plan = LaunchPlanner.plan(r.key, r.track, r.target, games, emulators, platform.ownPackage, r.data, r.extras)) {
            LaunchPlan.NothingToLaunch -> fail(str(R.string.launch_nothing))
            LaunchPlan.GameGone -> fail(str(R.string.launch_game_gone))
            is LaunchPlan.Track -> track(plan.cand, emulators)
            is LaunchPlan.Play -> {
                checking(plan.cand)
                platform.play(plan.cand, emulators).fold(
                    onSuccess = { finish(plan.cand, it) },
                    onFailure = { fail(str(R.string.msg_error, plan.cand.title, it.message.orEmpty())) },
                )
            }
            is LaunchPlan.Refused -> fail(str(R.string.launch_not_allowed, platform.appLabel(plan.pkg)))
            is LaunchPlan.Frontend -> {
                val label = platform.appLabel(plan.pkg)
                if (!platform.prepareForward(plan.target)) return fail(str(R.string.launch_not_startable, label))
                forwardReady = true
                forwardPkg = plan.pkg
                rom = plan.roms.firstOrNull()
                val match = plan.match
                if (match != null) {
                    playForward(match)
                } else {
                    _state.value = LaunchState.Choose(label, rom?.let(::fileName) ?: "", plan.candidates)
                }
            }
        }
    }

    /** The user said which game the file is (null: play without syncing). */
    fun choose(c: Forward.Candidate?) {
        if (!forwardReady) return
        viewModelScope.launch {
            if (c == null) {
                platform.startWithoutSync()?.let { return@launch fail(str(R.string.launch_failed, it)) }
                _state.value = LaunchState.Done
                return@launch
            }
            _state.value = LaunchState.Working(str(R.string.status_checking))
            // Remembered, so the next launch of this file finds the game.
            rom?.let { r ->
                if (c.profile.emulatorId != null) {
                    try {
                        core.setRom(c.profile.key, r)
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        platform.log("remember file", e)
                    }
                }
            }
            playForward(c)
        }
    }

    private fun checking(c: Forward.Candidate) {
        _state.value = LaunchState.Working(str(R.string.launch_checking_game, c.title))
    }

    private suspend fun playForward(c: Forward.Candidate) {
        checking(c)
        finish(c, platform.playForward(c, forwardPkg))
    }

    private fun finish(c: Forward.Candidate, out: PlayOutcome) {
        when (out) {
            is PlayOutcome.Started -> {
                platform.rememberRecent(c)
                _state.value = LaunchState.Done
            }
            is PlayOutcome.Cancelled -> _state.value = LaunchState.Done
            is PlayOutcome.Failed -> fail(out.message)
        }
    }

    /** The game is already running (its session had ended): sync and watch it again, starting nothing. */
    private suspend fun track(c: Forward.Candidate, emulators: List<EmulatorView>) {
        val pkg = Sessions.setup(c.profile, emulators)?.first?.let(Forward::packageOf).orEmpty()
        checking(c)
        when (val out = platform.track(c, pkg)) {
            is PlayOutcome.Started, is PlayOutcome.Cancelled -> _state.value = LaunchState.Done
            is PlayOutcome.Failed -> fail(out.message)
        }
    }

    private fun fileName(ref: String): String =
        Forward.normalize(ref)?.substringAfterLast('/')?.substringAfterLast(':') ?: ref
}
