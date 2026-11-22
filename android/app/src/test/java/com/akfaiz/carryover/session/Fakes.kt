package com.akfaiz.carryover.session

import com.akfaiz.carryover.core.BeginOutcome
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.EndOutcome
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.OpenSession
import com.akfaiz.carryover.core.SessionCore
import kotlinx.coroutines.CompletableDeferred

/** A stand-in for the Go core: answers as set, and records what it was asked. */
class FakeCore : SessionCore {
    var open: List<OpenSession> = emptyList()
    var pending: List<OpenSession> = emptyList()
    var games: List<GameView> = emptyList()
    var emulators: List<EmulatorView> = emptyList()

    /** Keys for which [sessionOpen] says a session is open. */
    val openKeys = mutableSetOf<String>()

    var begin = BeginOutcome(synced = true)
    var beginError: Exception? = null
    var end = EndOutcome()
    var endError: Exception? = null

    /** When set, [endSession] waits for it: the upload is "in progress". */
    var endGate: CompletableDeferred<Unit>? = null

    /** What finishing a pending session answers, by key (default: nothing to do). */
    val endPending = mutableMapOf<String, EndOutcome>()

    /** Pending sessions whose finishing throws. */
    val endPendingErrors = mutableMapOf<String, Exception>()

    val begun = mutableListOf<Pair<String, String>>()
    val ended = mutableListOf<String>()
    val endedPending = mutableListOf<String>()

    override suspend fun beginSession(key: String, emulator: String): BeginOutcome {
        begun += key to emulator
        beginError?.let { throw it }
        return begin
    }

    override suspend fun endSession(key: String): EndOutcome {
        ended += key
        endGate?.await()
        endError?.let { throw it }
        return end
    }

    override suspend fun openSessions() = open
    override suspend fun pendingSessions() = pending

    override suspend fun endPendingSession(key: String): EndOutcome {
        endedPending += key
        endPendingErrors[key]?.let { throw it }
        return endPending[key] ?: EndOutcome()
    }

    override fun sessionOpen(key: String) = key in openKeys
    var gamesError: Exception? = null
    var gamesCalls = 0

    override suspend fun games(): List<GameView> {
        gamesCalls++
        gamesError?.let { throw it }
        return games
    }

    override suspend fun emulators() = emulators
}

class FakeNotices : SessionNotices {
    val results = mutableListOf<Triple<String, String, Result<EndOutcome>>>()
    val offers = mutableListOf<String>()
    val cancelled = mutableListOf<String>()

    override fun result(title: String, key: String, result: Result<EndOutcome>) {
        results += Triple(title, key, result)
    }

    override fun offerTracking(key: String, title: String) {
        offers += key
    }

    override fun cancelOffer(key: String) {
        cancelled += key
    }
}

class FakeFront : FrontSource {
    var usage = true
    val events = mutableListOf<FrontEvent>()

    override fun hasUsageAccess() = usage
    override fun events(from: Long, to: Long) = events.filter { it.at in from..to }
}

class FakeHost : SessionHost {
    var changed = 0
    var idle = 0

    override fun changed() {
        changed++
    }

    override fun idle() {
        idle++
    }
}
