package com.akfaiz.carryover.ui.nav

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.setValue
import com.akfaiz.carryover.core.ArtKind

/** The app's pages: three tabs, and pages opened on top of them. */
sealed interface Screen {
    data object Library : Screen
    data object Emulators : Screen
    data object Settings : Screen
    data class Game(val id: String) : Screen
    data class Emulator(val id: String) : Screen
    data object Cloud : Screen
    data object Trash : Screen

    /** The backups of one profile of a game (a game id, or `<game>.<emulator>`). */
    data class Backups(val key: String) : Screen

    /** Finding a cover or a background for a game on SteamGridDB. */
    data class FindArt(val id: String, val kind: ArtKind) : Screen
    data object AddRemote : Screen
    data object AddGame : Screen
    data class EditGame(val id: String) : Screen

    /** How to start games from a frontend, with the settings made for an emulator ("" = the first). */
    data class Frontends(val emulatorId: String = "") : Screen
}

/** A page at its place: how many pages are open over the tab, itself included (0 is a tab). */
data class Place(val screen: Screen, val depth: Int) {
    /** What the page's saved state (scroll position, a search) is kept under while it can be returned to. */
    val stateKey: String get() = "$depth/$screen"
}

/** Which way one page gives way to another. */
enum class PageMove { Fade, Forward, Back }

/** Opening a page over another goes forward and leaving it goes back; one tab for another is neither. */
fun pageMove(from: Place, to: Place): PageMove = when {
    to.depth > from.depth -> PageMove.Forward
    to.depth < from.depth -> PageMove.Back
    else -> PageMove.Fade
}

/** Where the user is: a tab, and the pages opened over it (the last one is shown). */
class Nav {
    var tab by mutableStateOf<Screen>(Screen.Library)
        private set
    val stack = mutableStateListOf<Screen>()

    /** Set by a page that needs the SteamGridDB key: Settings opens its dialog once and clears it. */
    var askGridKey by mutableStateOf(false)
    val current: Screen get() = stack.lastOrNull() ?: tab
    val canGoBack: Boolean get() = stack.isNotEmpty() || tab != Screen.Library

    /** The page shown, at its place. */
    val place: Place get() = Place(current, stack.size)

    /** The state keys of the pages still open: the tabs, and what can be gone back to. */
    val openStateKeys: Set<String>
        get() = buildSet {
            for (t in listOf(Screen.Library, Screen.Emulators, Screen.Settings)) add(Place(t, 0).stateKey)
            stack.forEachIndexed { i, s -> add(Place(s, i + 1).stateKey) }
        }

    fun go(s: Screen) {
        if (s == Screen.Library || s == Screen.Emulators || s == Screen.Settings) {
            stack.clear()
            tab = s
        } else {
            stack.add(s)
        }
    }

    fun back() {
        when {
            stack.isNotEmpty() -> stack.removeAt(stack.lastIndex)
            tab != Screen.Library -> tab = Screen.Library
        }
    }

    companion object {
        private fun encode(s: Screen): String = when (s) {
            Screen.Library -> "library"
            Screen.Emulators -> "emulators"
            Screen.Settings -> "settings"
            is Screen.Game -> "game:${s.id}"
            is Screen.Emulator -> "emulator:${s.id}"
            Screen.Cloud -> "cloud"
            Screen.Trash -> "trash"
            is Screen.Backups -> "backups:${s.key}"
            is Screen.FindArt -> "findArt:${s.id}:${s.kind.wire}"
            Screen.AddRemote -> "addRemote"
            Screen.AddGame -> "addGame"
            is Screen.EditGame -> "editGame:${s.id}"
            is Screen.Frontends -> "frontends:${s.emulatorId}"
        }

        private fun decode(v: String): Screen? = when {
            v == "library" -> Screen.Library
            v == "emulators" -> Screen.Emulators
            v == "settings" -> Screen.Settings
            v.startsWith("game:") -> Screen.Game(v.removePrefix("game:"))
            v.startsWith("emulator:") -> Screen.Emulator(v.removePrefix("emulator:"))
            v == "cloud" -> Screen.Cloud
            v == "trash" -> Screen.Trash
            v.startsWith("backups:") -> Screen.Backups(v.removePrefix("backups:"))
            v.startsWith("findArt:") -> v.removePrefix("findArt:").let { rest ->
                val kind = ArtKind.entries.firstOrNull { it.wire == rest.substringAfterLast(':') } ?: ArtKind.Cover
                Screen.FindArt(rest.substringBeforeLast(':'), kind)
            }
            v == "addRemote" -> Screen.AddRemote
            v == "addGame" -> Screen.AddGame
            v.startsWith("editGame:") -> Screen.EditGame(v.removePrefix("editGame:"))
            v.startsWith("frontends:") -> Screen.Frontends(v.removePrefix("frontends:"))
            else -> null
        }

        /** Keeps the place across activity restarts (rotation, a new language). */
        val Saver: Saver<Nav, Any> = listSaver(
            save = { nav -> listOf(encode(nav.tab)) + nav.stack.map(::encode) },
            restore = { saved ->
                Nav().apply {
                    saved.mapNotNull(::decode).forEachIndexed { i, s -> if (i == 0) go(s) else stack.add(s) }
                }
            },
        )
    }
}
