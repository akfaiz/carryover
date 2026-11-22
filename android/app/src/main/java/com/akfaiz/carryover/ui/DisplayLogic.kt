package com.akfaiz.carryover.ui

import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.data.MAX_BACKUPS
import java.time.Instant
import java.time.OffsetDateTime
import kotlin.math.ceil

// How things are shown, decided without Compose: unit tested (DisplayLogicTest).

/** How a profile or game stands, from the most to the least urgent. */
enum class Badge { Playing, Busy, Conflict, Error, Changed, Synced, Never, NotSetUp }

/** The state of one profile on this device. */
fun badgeOf(p: Profile, state: UiState, ending: Set<String>): Badge = when {
    p.status.running -> Badge.Playing
    p.key in state.syncing || p.key in state.starting || p.key in ending -> Badge.Busy
    !p.canSync -> Badge.NotSetUp
    p.status.conflict -> Badge.Conflict
    p.status.lastError.isNotEmpty() -> Badge.Error
    p.status.localChanges -> Badge.Changed
    p.status.lastSyncAt.isNotEmpty() -> Badge.Synced
    else -> Badge.Never
}

/** A game's state: that of its most urgent profile. */
fun badgeOf(g: GameView, state: UiState, ending: Set<String>): Badge =
    g.profiles.minOfOrNull { badgeOf(it, state, ending) } ?: Badge.NotSetUp

/** What the user calls a profile: the game's name, with its emulator in brackets. */
fun profileTitle(g: GameView, p: Profile): String = if (p.emulator == null) g.name else "${g.name} (${p.emulator})"

/** The letters of a game's cover placeholder: the first of its first two words, or two of its only word. */
fun initials(name: String): String {
    val words = name.split(' ', '-', ':', '_').filter { it.isNotBlank() && it.first().isLetterOrDigit() }
    return when {
        words.isEmpty() -> "?"
        words.size == 1 -> words[0].take(2).uppercase()
        else -> (words[0].take(1) + words[1].take(1)).uppercase()
    }
}

/** A size in bytes, as KB or MB from 1 KB on. */
fun formatBytes(n: Long): String = when {
    n >= 1 shl 20 -> "%.1f MB".format(n / 1048576.0)
    n >= 1 shl 10 -> "%.1f KB".format(n / 1024.0)
    else -> "$n B"
}

/** A name for a new remote that no other has: the type (gdrive for Google Drive), then type2, type3… */
fun defaultRemoteName(type: String, taken: Set<String>): String {
    val base = if (type == "drive") "gdrive" else type
    if (base !in taken) return base
    var i = 2
    while ("$type$i" in taken) i++
    return "$type$i"
}

/**
 * How many days are left until [expiresAt] (RFC 3339), counting a started day as a day, or null when it can't be
 * read. 0 means it is about to be removed.
 */
fun daysLeft(expiresAt: String, now: Instant = Instant.now()): Int? {
    val at = runCatching { OffsetDateTime.parse(expiresAt).toInstant() }.getOrNull() ?: return null
    val hours = (at.epochSecond - now.epochSecond) / 3600.0
    return maxOf(0, ceil(hours / 24).toInt())
}

/**
 * Whether [url] is an image on SteamGridDB's own hosts over https: the only
 * images the app fetches itself (the core downloads the chosen one by the same
 * rule).
 */
fun isGridImageUrl(url: String): Boolean {
    val uri = runCatching { java.net.URI(url) }.getOrNull() ?: return false
    if (uri.scheme != "https" || uri.userInfo != null) return false
    val host = uri.host?.lowercase() ?: return false
    return host == "steamgriddb.com" || host.endsWith(".steamgriddb.com")
}

/**
 * The choices for how many cloud backups to keep: off (0), a few usual
 * numbers, and the current one when it is none of them (the desktop app takes
 * any number from 1 to 100).
 */
fun backupChoices(current: Int): List<Int> = (listOf(0, 3, 5, 10, 20) + current.coerceIn(0, MAX_BACKUPS)).distinct().sorted()

/**
 * The filters of a suggested save folder as the screen shows them: the file patterns, without the twin ending in a
 * slash and two stars that the core adds to each so that a folder of that name is taken too.
 */
fun suggestionFilters(include: List<String>): List<String> = include.filter { !it.endsWith("/" + "**") }.distinct()
