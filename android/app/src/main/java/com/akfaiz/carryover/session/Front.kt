package com.akfaiz.carryover.session

/** Timings and packages of the session detector. */
object FrontPolicy {
    /** How long a game may be out of front before its session ends (the default; the user can change it). */
    const val GRACE_MS = 10_000L

    /** A game that never came to the front within this long after its session started is not running. */
    const val NEVER_STARTED_MS = 60_000L

    /** Clock difference allowed between a session's start and the game's first event. */
    const val EVENT_SLACK_MS = 2_000L

    /**
     * Screens that come to the front over a game for a moment and don't mean
     * the user left it: system dialogs and pickers, permission prompts, the
     * sign-in of Google Play Games. The game's screen returns after them.
     */
    val SYSTEM_SCREENS = setOf(
        "android",
        "com.android.systemui",
        "com.android.intentresolver",
        "com.android.permissioncontroller",
        "com.google.android.permissioncontroller",
        "com.android.documentsui",
        "com.google.android.documentsui",
        "com.android.packageinstaller",
        "com.google.android.packageinstaller",
        "com.google.android.gms",
        "com.google.android.gsf",
    )
}

/**
 * Where a watched game is, judged from the screens that came to the front
 * (Usage access): pure data, unit tested without Android. [onFront] is fed
 * every screen that came to the front, oldest first; [verdict] says whether
 * the session is over.
 */
class FrontState(
    val pkg: String,
    val startedAt: Long,
    var everFront: Boolean = false,
    var leftAt: Long? = null,
) {
    enum class Verdict { KEEP, LEFT, NEVER_STARTED }

    /** The screen of [front] came to the front at [at]. */
    fun onFront(front: String, at: Long) {
        if (at < startedAt - FrontPolicy.EVENT_SLACK_MS) return // before this session
        when {
            front == pkg -> {
                everFront = true
                leftAt = null
            }
            front in FrontPolicy.SYSTEM_SCREENS -> {} // the game is still there behind it
            everFront && leftAt == null -> leftAt = at
        }
    }

    /**
     * Whether the session is over at [now]: LEFT when another app's screen
     * came to the front and the game's didn't come back within [graceMs];
     * NEVER_STARTED when, with Usage access, the game didn't come to the
     * front at all.
     */
    fun verdict(now: Long, hasUsageAccess: Boolean, graceMs: Long = FrontPolicy.GRACE_MS): Verdict {
        val left = leftAt
        return when {
            left != null && now - left >= graceMs -> Verdict.LEFT
            !everFront && hasUsageAccess && now - startedAt > FrontPolicy.NEVER_STARTED_MS -> Verdict.NEVER_STARTED
            else -> Verdict.KEEP
        }
    }
}
