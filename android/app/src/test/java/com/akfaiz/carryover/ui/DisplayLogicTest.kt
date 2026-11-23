package com.akfaiz.carryover.ui

import androidx.compose.runtime.saveable.SaverScope
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.ProfileView
import com.akfaiz.carryover.core.SaveSlot
import com.akfaiz.carryover.core.SyncEvent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant
import java.util.Locale

/** How things are shown, decided without Compose. */
class DisplayLogicTest {
    private fun profile(
        key: String = "g",
        emulator: String? = null,
        status: GameStatus = GameStatus(),
        ready: Boolean = true,
    ) = Profile(
        key = key,
        emulator = emulator,
        emulatorId = emulator?.lowercase(),
        slots = listOf(SaveSlot("saves")),
        paths = if (ready) mapOf("saves" to "/x") else emptyMap(),
        resolvedPaths = if (ready) mapOf("saves" to "/x") else emptyMap(),
        pathErrors = emptyMap(),
        status = status,
    )

    private val idle = UiState()

    @Test
    fun aProfileStandsAsItsMostUrgentStateSays() {
        assertEquals(Badge.Playing, badgeOf(profile(status = GameStatus(running = true, conflict = true)), idle, emptySet()))
        assertEquals(Badge.Conflict, badgeOf(profile(status = GameStatus(conflict = true, lastError = "x")), idle, emptySet()))
        assertEquals(Badge.Error, badgeOf(profile(status = GameStatus(lastError = "x", localChanges = true)), idle, emptySet()))
        assertEquals(Badge.Changed, badgeOf(profile(status = GameStatus(localChanges = true, lastSyncAt = "t")), idle, emptySet()))
        assertEquals(Badge.Synced, badgeOf(profile(status = GameStatus(lastSyncAt = "t")), idle, emptySet()))
        assertEquals(Badge.Never, badgeOf(profile(), idle, emptySet()))
    }

    @Test
    fun aProfileNotSetUpHereIsNotSetUpWhateverElseItHas() {
        assertEquals(Badge.NotSetUp, badgeOf(profile(ready = false, status = GameStatus(conflict = true)), idle, emptySet()))
    }

    @Test
    fun aBusyProfileIsBusyWhileItSyncsStartsOrEnds() {
        val syncing = UiState(syncing = mapOf("g" to SyncEvent()))
        val starting = UiState(starting = setOf("g"))
        assertEquals(Badge.Busy, badgeOf(profile(), syncing, emptySet()))
        assertEquals(Badge.Busy, badgeOf(profile(), starting, emptySet()))
        assertEquals(Badge.Busy, badgeOf(profile(), idle, setOf("g")))
        assertEquals("other keys don't count", Badge.Never, badgeOf(profile(), idle, setOf("h")))
        assertEquals("playing beats busy", Badge.Playing, badgeOf(profile(status = GameStatus(running = true)), syncing, emptySet()))
    }

    @Test
    fun aGameStandsAsItsMostUrgentProfile() {
        val g = GameView(
            id = "g",
            name = "Zelda",
            emulators = listOf(
                ProfileView(emulatorId = "a", name = "A", key = "g.a", slots = listOf(SaveSlot("s")), resolvedPaths = mapOf("s" to "/x"), status = GameStatus(lastSyncAt = "t")),
                ProfileView(emulatorId = "b", name = "B", key = "g.b", slots = listOf(SaveSlot("s")), resolvedPaths = mapOf("s" to "/x"), status = GameStatus(conflict = true)),
            ),
        )
        assertEquals(Badge.Conflict, badgeOf(g, idle, emptySet()))
        assertEquals(Badge.NotSetUp, badgeOf(GameView(id = "e", name = "Empty"), idle, emptySet()))
    }

    @Test
    fun theBadgeOrderIsMostUrgentFirst() {
        assertEquals(
            listOf(Badge.Playing, Badge.Busy, Badge.Conflict, Badge.Error, Badge.Changed, Badge.Synced, Badge.Never, Badge.NotSetUp),
            Badge.entries,
        )
    }

    @Test
    fun aProfileIsNamedWithItsEmulator() {
        val g = GameView(id = "g", name = "Zelda")
        assertEquals("Zelda", profileTitle(g, profile()))
        assertEquals("Zelda (Eden)", profileTitle(g, profile(emulator = "Eden")))
    }

    @Test
    fun initialsOfACoverPlaceholder() {
        assertEquals("SM", initials("Super Mario World"))
        assertEquals("ZT", initials("Zelda: Tears of the Kingdom"))
        assertEquals("ZE", initials("Zelda"))
        assertEquals("A", initials("a"))
        assertEquals("OT", initials("one-two_three"))
        assertEquals("?", initials(""))
        assertEquals("?", initials("  - : "))
        assertEquals("SM", initials("- Super Mario"))
        assertEquals("1S", initials("1942 Strike"))
    }

    @Test
    fun sizesInBytesKbAndMb() {
        val old = Locale.getDefault()
        Locale.setDefault(Locale.US)
        try {
            assertEquals("0 B", formatBytes(0))
            assertEquals("1023 B", formatBytes(1023))
            assertEquals("1.0 KB", formatBytes(1024))
            assertEquals("1.5 KB", formatBytes(1536))
            assertEquals("1.0 MB", formatBytes(1L shl 20))
            assertEquals("12.5 MB", formatBytes(13_107_200))
        } finally {
            Locale.setDefault(old)
        }
    }

    @Test
    fun aNewRemoteGetsAFreeName() {
        assertEquals("gdrive", defaultRemoteName("drive", emptySet()))
        assertEquals("webdav", defaultRemoteName("webdav", emptySet()))
        assertEquals("webdav2", defaultRemoteName("webdav", setOf("webdav")))
        assertEquals("webdav3", defaultRemoteName("webdav", setOf("webdav", "webdav2")))
        // Google Drive's type is "drive", its first name "gdrive", the next "drive2".
        assertEquals("drive2", defaultRemoteName("drive", setOf("gdrive")))
    }

    @Test
    fun onlySteamGridDBImagesOverHttpsAreFetched() {
        assertTrue(isGridImageUrl("https://cdn2.steamgriddb.com/grid/abc.png"))
        assertTrue(isGridImageUrl("https://steamgriddb.com/x.png"))
        assertTrue(isGridImageUrl("https://CDN.SteamGridDB.com/x.png"))
        assertFalse("not https", isGridImageUrl("http://cdn2.steamgriddb.com/x.png"))
        assertFalse("another host", isGridImageUrl("https://example.com/x.png"))
        assertFalse("a host that only ends like it", isGridImageUrl("https://evilsteamgriddb.com/x.png"))
        assertFalse("a lookalike subdomain of another host", isGridImageUrl("https://steamgriddb.com.evil.com/x.png"))
        assertFalse("credentials in the URL", isGridImageUrl("https://user:pw@cdn2.steamgriddb.com/x.png"))
        assertFalse(isGridImageUrl(""))
        assertFalse(isGridImageUrl("not a url"))
        assertFalse(isGridImageUrl("file:///sdcard/x.png"))
    }

    // How long a deleted game can still be restored

    private val now = Instant.parse("2026-11-22T10:00:00Z")

    @Test
    fun daysLeftCountsAStartedDayAsADay() {
        assertEquals(30, daysLeft("2026-12-22T10:00:00Z", now))
        assertEquals(29, daysLeft("2026-12-21T10:00:00Z", now))
        assertEquals("a part of a day", 1, daysLeft("2026-11-22T10:00:01Z", now))
        assertEquals(1, daysLeft("2026-11-23T10:00:00Z", now))
        assertEquals(2, daysLeft("2026-11-23T10:00:01Z", now))
    }

    @Test
    fun daysLeftIsZeroWhenItIsDueOrPast() {
        assertEquals(0, daysLeft("2026-11-22T10:00:00Z", now))
        assertEquals(0, daysLeft("2026-11-01T10:00:00Z", now))
    }

    @Test
    fun daysLeftReadsTheCoresTimeFormats() {
        // Go writes RFC 3339 with nanoseconds and the zone it was made in.
        assertEquals(30, daysLeft("2026-12-22T10:00:00.123456789Z", now))
        assertEquals(30, daysLeft("2026-12-22T17:00:00+07:00", now))
        assertEquals("the same instant in another zone", daysLeft("2026-12-22T10:00:00Z", now), daysLeft("2026-12-22T17:00:00+07:00", now))
    }

    @Test
    fun daysLeftIsNullWhenTheTimeCantBeRead() {
        assertNull(daysLeft("", now))
        assertNull(daysLeft("soon", now))
        assertNull(daysLeft("2026-12-22", now))
    }

    // The place in the app is kept across a restart (rotation, a new language)

    private fun roundTrip(nav: Nav): Nav {
        val saved = with(Nav.Saver) { SaverScope { true }.save(nav) }
        assertNotNull(saved)
        return Nav.Saver.restore(saved!!)!!
    }

    @Test
    fun theTabAndThePagesOnTopAreRestored() {
        val nav = Nav()
        nav.go(Screen.Settings)
        nav.go(Screen.Cloud)
        nav.go(Screen.AddRemote)
        nav.go(Screen.Game("g1"))
        nav.go(Screen.EditGame("g1"))
        nav.go(Screen.Emulator("e1"))
        nav.go(Screen.Frontends("e1"))
        nav.go(Screen.Trash)
        nav.go(Screen.FindArt("g1", com.akfaiz.carryover.core.ArtKind.Background))
        nav.go(Screen.Backups("g1.e1"))
        nav.go(Screen.AddGame)
        val back = roundTrip(nav)
        assertEquals(Screen.Settings, back.tab)
        assertEquals(nav.stack.toList(), back.stack.toList())
        assertEquals(Screen.AddGame, back.current)
    }

    @Test
    fun aFreshNavIsOnTheLibrary() {
        val back = roundTrip(Nav())
        assertEquals(Screen.Library, back.tab)
        assertTrue(back.stack.isEmpty())
        assertFalse(back.canGoBack)
    }

    @Test
    fun anUnknownPageInASavedStateIsDropped() {
        val back = Nav.Saver.restore(listOf("emulators", "game:g1", "page-from-the-future", "cloud"))!!
        assertEquals(Screen.Emulators, back.tab)
        assertEquals(listOf(Screen.Game("g1"), Screen.Cloud), back.stack.toList())
    }

    @Test
    fun goingToATabClearsThePagesAndBackWalksOut() {
        val nav = Nav()
        nav.go(Screen.Game("g"))
        nav.go(Screen.EditGame("g"))
        assertEquals(Screen.EditGame("g"), nav.current)
        nav.back()
        assertEquals(Screen.Game("g"), nav.current)
        nav.go(Screen.Emulators)
        assertTrue("a tab replaces the pages", nav.stack.isEmpty())
        assertTrue(nav.canGoBack)
        nav.back()
        assertEquals("back from another tab goes to the library", Screen.Library, nav.tab)
        assertFalse(nav.canGoBack)
        nav.back() // nothing to go back to: stays
        assertEquals(Screen.Library, nav.current)
    }

    @Test
    fun backupChoicesAreOffAFewUsualNumbersAndTheCurrentOne() {
        assertEquals(listOf(0, 3, 5, 10, 20), backupChoices(0))
        assertEquals(listOf(0, 3, 5, 10, 20), backupChoices(5))
        assertEquals(listOf(0, 3, 5, 7, 10, 20), backupChoices(7))
        assertEquals(listOf(0, 3, 5, 10, 20, 100), backupChoices(100))
        assertEquals("a number from outside the range is not offered", listOf(0, 3, 5, 10, 20, 100), backupChoices(500))
        assertEquals(listOf(0, 3, 5, 10, 20), backupChoices(-4))
    }

    @Test
    fun suggestionFiltersAreThePatternsWithoutTheirFolderTwins() {
        assertEquals(listOf("*.gci"), suggestionFilters(listOf("*.gci", "*.gci/" + "**")))
        assertEquals(listOf("a", "b"), suggestionFilters(listOf("a", "a/" + "**", "b", "b/" + "**", "a")))
        assertEquals(emptyList<String>(), suggestionFilters(emptyList()))
    }

    @Test
    fun filtersAreEditedOnePerLine() {
        assertEquals(listOf("*.sav", "saves/" + "**"), parseFilters("  *.sav \n\n saves/" + "**\n"))
        assertEquals(emptyList<String>(), parseFilters("  \n"))
        assertEquals("a\nb", filtersText(listOf("a", "b")))
        assertEquals(listOf("a", "b"), parseFilters(filtersText(listOf("a", "b"))))
    }

    @Test
    fun aSummaryLeavesOutTheFolderTwinOfAPatternItShows() {
        assertEquals(listOf("ULUS10067_*"), shownFilters(listOf("ULUS10067_*", "ULUS10067_*/" + "**")))
        assertEquals("a twin alone is the pattern", listOf("saves/" + "**"), shownFilters(listOf("saves/" + "**")))
        assertEquals(emptyList<String>(), shownFilters(emptyList()))
    }
}
