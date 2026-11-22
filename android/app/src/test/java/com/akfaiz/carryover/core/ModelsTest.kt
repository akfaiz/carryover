package com.akfaiz.carryover.core

import org.junit.Assert.assertEquals
import org.junit.Test

/** The core's JSON, decoded into typed values: what an old app does with a value it doesn't know. */
class ModelsTest {
    private fun outcome(action: String) = json.decodeFromString<SyncOutcome>("""{"action":"$action"}""")

    @Test
    fun syncActionsDecode() {
        assertEquals(SyncAction.Push, outcome("push").action)
        assertEquals(SyncAction.Pull, outcome("pull").action)
        assertEquals(SyncAction.Conflict, outcome("conflict").action)
        assertEquals(SyncAction.None, outcome("none").action)
    }

    @Test
    fun anUnknownOrMissingActionIsNone() {
        assertEquals(SyncAction.None, outcome("merge-somehow").action)
        assertEquals(SyncAction.None, outcome("").action)
        assertEquals(SyncAction.None, json.decodeFromString<SyncOutcome>("{}").action)
        assertEquals(SyncAction.None, json.decodeFromString<SyncOutcome>("""{"action":null}""").action)
    }

    @Test
    fun syncResultsAndPlansUseTheSameActions() {
        val all = json.decodeFromString<SyncAllOutcome>("""{"results":[{"gameId":"a","action":"conflict"},{"gameId":"b","action":"pull"}]}""")
        assertEquals(listOf(SyncAction.Conflict, SyncAction.Pull), all.results.map { it.action })
        assertEquals(SyncAction.Push, json.decodeFromString<PlanView>("""{"action":"push"}""").action)
    }

    @Test
    fun noticeKindsDecode() {
        fun kind(k: String) = json.decodeFromString<Notice>("""{"kind":"$k"}""").kind
        assertEquals(NoticeKind.Uploaded, kind("uploaded"))
        assertEquals(NoticeKind.Conflict, kind("conflict"))
        assertEquals(NoticeKind.Failed, kind("failed"))
        assertEquals(NoticeKind.Offline, kind("offline"))
        assertEquals(NoticeKind.Unknown, kind("something new"))
        assertEquals(NoticeKind.Unknown, json.decodeFromString<Notice>("{}").kind)
    }

    @Test
    fun promptKindsDecode() {
        fun kind(k: String) = json.decodeFromString<Prompt>("""{"id":"1","kind":"$k"}""").kind
        assertEquals(PromptKind.Conflict, kind("conflict"))
        assertEquals(PromptKind.Lock, kind("lock"))
        assertEquals(PromptKind.SyncError, kind("syncError"))
        assertEquals(PromptKind.Emulator, kind("emulator"))
        assertEquals(PromptKind.Unknown, kind("a question from the future"))
    }

    @Test
    fun theTrashDecodesWithItsRetentionAndLists() {
        val t = json.decodeFromString<TrashView>(
            """{"days":30,"games":[{"id":"g1","name":"Zelda","deletedAt":"2026-11-20T10:00:00Z","expiresAt":"2026-12-20T10:00:00Z","emulators":2}],"emulators":[{"id":"e1","name":"Eden","deletedAt":"d","expiresAt":"x"}]}""",
        )
        assertEquals(30, t.days)
        assertEquals("Zelda", t.games.single().name)
        assertEquals(2, t.games.single().emulators)
        assertEquals("Eden", t.emulators.single().name)
    }

    @Test
    fun anEmptyOrNullTrashDecodesToEmptyLists() {
        val t = json.decodeFromString<TrashView>("""{"days":30,"games":null,"emulators":null}""")
        assertEquals(emptyList<TrashGame>(), t.games)
        assertEquals(emptyList<TrashEmulator>(), t.emulators)
        assertEquals(30, TrashView().days)
    }

    @Test
    fun leftoversDecodeAndAreNamedWithTheirEmulator() {
        val l = json.decodeFromString<List<Leftover>>(
            """[{"key":"k","gameId":"g","gameName":"Zelda","emulatorId":"e","emulatorName":"Eden","slots":["saves"],"files":3,"bytes":2048,"time":"2026-11-20T10:00:00Z","backups":1},{"key":"k2","gameId":"g2","gameName":"Mario","slots":null}]""",
        )
        assertEquals("Zelda (Eden)", l[0].title)
        assertEquals(3, l[0].files)
        assertEquals(2048L, l[0].bytes)
        assertEquals("Mario", l[1].title)
        assertEquals(emptyList<String>(), l[1].slots)
    }

    @Test
    fun aGameSaysWhetherThisDeviceKnowsIt() {
        assertEquals(true, json.decodeFromString<GameView>("""{"id":"g","hasDevice":true}""").hasDevice)
        assertEquals(false, json.decodeFromString<GameView>("""{"id":"g"}""").hasDevice)
    }

    @Test
    fun steamGridDBGamesAndImagesDecode() {
        val games = json.decodeFromString<List<GridGame>>("""[{"id":7,"name":"Zelda","year":2017,"verified":true},{"id":8,"name":"Mario"}]""")
        assertEquals(GridGame(7, "Zelda", 2017, true), games[0])
        assertEquals(false, games[1].verified)
        val art = json.decodeFromString<List<GridArt>>(
            """[{"id":1,"url":"https://cdn2.steamgriddb.com/a.png","thumb":"https://cdn2.steamgriddb.com/t.png","width":600,"height":900,"style":"alternate","score":9,"author":"someone"}]""",
        )
        assertEquals(600, art.single().width)
        assertEquals("someone", art.single().author)
        assertEquals(emptyList<GridArt>(), json.decodeFromString<List<GridArt>>("[]"))
    }

    @Test
    fun theArtKindsAreTheCoresWords() {
        assertEquals(listOf("cover", "hero"), ArtKind.entries.map { it.wire })
    }

    @Test
    fun backupsDecodeWithTheirKindAndFlags() {
        val list = json.decodeFromString<List<Backup>>(
            """[{"name":"a.zip","time":"2026-11-22T10:00:00.123456Z","deviceName":"Pixel","kind":"manual","pinned":true,"local":false},
                {"name":"b.zip","time":"2026-11-21T10:00:00Z","deviceName":"PC","kind":"local"},
                {"name":"c.zip","deviceName":"PC","kind":"safety","local":true},
                {"name":"d.zip","kind":"from-the-future"}]""",
        )
        assertEquals(BackupKind.Manual, list[0].kind)
        assertEquals(true, list[0].pinned)
        assertEquals(BackupKind.Local, list[1].kind)
        assertEquals(false, list[1].pinned)
        assertEquals(BackupKind.Safety, list[2].kind)
        assertEquals(true, list[2].local)
        assertEquals("a kind this app doesn't know reads as a cloud backup", BackupKind.Cloud, list[3].kind)
        assertEquals(emptyList<Backup>(), json.decodeFromString<List<Backup>>("[]"))
    }

    @Test
    fun anImportResultDecodes() {
        val r = json.decodeFromString<ImportResult>("""{"imported":["a","b"],"skipped":["c (s3)"],"replaced":["a"]}""")
        assertEquals(listOf("a", "b"), r.imported)
        assertEquals(listOf("c (s3)"), r.skipped)
        assertEquals(listOf("a"), r.replaced)
        assertEquals(ImportResult(), json.decodeFromString<ImportResult>("""{"imported":null,"skipped":null,"replaced":null}"""))
    }

    @Test
    fun saveSuggestionsDecodeAndEncodeBackForTheCore() {
        val res = json.decodeFromString<SaveSuggestions>(
            """{"suggestions":[{"game":"Luigi","path":":saf:a/b/GC","include":["*.gci","*.gci/**"],"exists":true},
                {"game":"Odyssey (account save)","path":"/p","kind":"account"},{"game":"x","path":"/q","kind":"from-the-future"}],
                "perGame":true,"note":"n"}""",
        )
        assertEquals(true, res.perGame)
        assertEquals("n", res.note)
        assertEquals(SaveKind.None, res.suggestions[0].kind)
        assertEquals(true, res.suggestions[0].exists)
        assertEquals(listOf("*.gci", "*.gci/**"), res.suggestions[0].include)
        assertEquals(SaveKind.Account, res.suggestions[1].kind)
        assertEquals("a kind this app doesn't know is no kind", SaveKind.None, res.suggestions[2].kind)
        // What goes back to the core keeps the words it uses: kind "" for none, "account" for an account save.
        val back = json.encodeToString(res.suggestions[1])
        assertEquals(true, "\"kind\":\"account\"" in back)
        assertEquals(true, "\"kind\":\"\"" in json.encodeToString(res.suggestions[0]))
        assertEquals(SaveSuggestions(), json.decodeFromString<SaveSuggestions>("{}"))
        assertEquals(3, json.decodeFromString<AutofillResult>("""{"applied":3}""").applied)
    }

    @Test
    fun theSettingsCarryTheLibrarySharing() {
        val st = json.decodeFromString<Settings>("""{"syncLibrary":true,"coversLocalOnly":true}""")
        assertEquals(true, st.syncLibrary)
        assertEquals(true, st.coversLocalOnly)
        assertEquals(false, json.decodeFromString<Settings>("{}").coversLocalOnly)
    }

    @Test
    fun theSettingsCarryTheSteamGridDBKey() {
        assertEquals("k", json.decodeFromString<Settings>("""{"steamGridDbKey":"k"}""").steamGridDbKey)
        assertEquals("", json.decodeFromString<Settings>("{}").steamGridDbKey)
    }

    @Test
    fun answersAreWhatTheCoreExpects() {
        // mobile.Respond and Sync take these exact words.
        assertEquals(
            listOf("local", "cloud", "skip", "yes", "no", "cancel"),
            Answer.entries.map { it.wire },
        )
        assertEquals(listOf("local", "cloud"), Resolution.entries.map { it.wire })
    }

    @Test
    fun theEnumsCanBeEncodedAgain() {
        assertEquals(""""push"""", json.encodeToString(SyncAction.serializer(), SyncAction.Push))
    }
}
