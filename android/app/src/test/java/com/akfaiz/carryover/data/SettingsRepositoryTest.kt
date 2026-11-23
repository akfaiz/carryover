package com.akfaiz.carryover.data

import com.akfaiz.carryover.core.DeviceView
import com.akfaiz.carryover.core.Remote
import com.akfaiz.carryover.core.Settings
import com.akfaiz.carryover.core.SettingsCore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

internal class FakeSettingsCore : SettingsCore {
    var settings = Settings(deviceName = "Phone", theme = "dark")
    var remotes = listOf(Remote("gdrive", "drive"))
    val calls = mutableListOf<String>()
    val updates = mutableListOf<Map<String, JsonElement>>()
    var testError: Exception? = null
    var updateError: Exception? = null

    var settingsError: Exception? = null

    override suspend fun settings(): Settings {
        calls += "settings"
        settingsError?.let { throw it }
        return settings
    }

    override suspend fun updateSettings(changes: Map<String, JsonElement>): Settings {
        calls += "update"
        updateError?.let { throw it }
        updates += changes
        settings = settings.copy(
            theme = changes["theme"]?.jsonPrimitive?.contentOrNull ?: settings.theme,
            deviceName = changes["deviceName"]?.jsonPrimitive?.contentOrNull ?: settings.deviceName,
            remote = changes["remote"]?.jsonPrimitive?.contentOrNull ?: settings.remote,
            remoteRoot = changes["remoteRoot"]?.jsonPrimitive?.contentOrNull ?: settings.remoteRoot,
            syncLibrary = changes["syncLibrary"]?.jsonPrimitive?.booleanOrNull ?: settings.syncLibrary,
            steamGridDbKey = changes["steamGridDbKey"]?.jsonPrimitive?.contentOrNull ?: settings.steamGridDbKey,
            backupRetention = changes["backupRetention"]?.jsonPrimitive?.intOrNull ?: settings.backupRetention,
            coversLocalOnly = changes["coversLocalOnly"]?.jsonPrimitive?.booleanOrNull ?: settings.coversLocalOnly,
        )
        return settings
    }

    override suspend fun remotes(): List<Remote> {
        calls += "remotes"
        return remotes
    }

    var deleteError: Exception? = null
    var keyError: Exception? = null

    override suspend fun deleteRemote(name: String) {
        calls += "delete $name"
        deleteError?.let { throw it }
        remotes = remotes.filter { it.name != name }
    }

    override suspend fun testSteamGridKey(key: String) {
        calls += "testKey $key"
        keyError?.let { throw it }
    }

    override suspend fun testRemote(remote: String, root: String) {
        calls += "test $remote:$root"
        testError?.let { throw it }
    }

    var libraryError: Exception? = null

    override suspend fun syncLibrary() {
        calls += "syncLibrary"
        libraryError?.let { throw it }
    }

    var devices = listOf(DeviceView(id = "this", name = "Phone", isThis = true))
    var deviceError: Exception? = null

    override suspend fun devices(): List<DeviceView> {
        calls += "devices"
        return devices
    }

    override suspend fun forgetDevice(id: String) {
        calls += "forget $id"
        deviceError?.let { throw it }
        devices = devices.filter { it.id != id }
    }

    /** As the core does: this device takes over [id]'s settings and keeps its own id, and [id] goes. */
    override suspend fun reuseDevice(id: String) {
        calls += "reuse $id"
        deviceError?.let { throw it }
        val taken = devices.firstOrNull { it.id == id }
        devices = devices.filter { it.id != id }.map { if (it.isThis && taken != null) it.copy(games = it.games + taken.games) else it }
    }
}

internal class FakePrefs : PreferenceStore {
    override val graceChoices = listOf(10, 30, 60)
    override var theme = "light"
    override var wallpaper = false
    override var graceSeconds = 30
    override var backgroundSync = false
}

class SettingsRepositoryTest {
    private val core = FakeSettingsCore()
    private val prefs = FakePrefs()
    private var applied = mutableListOf<Boolean>()
    private val repo = SettingsRepository(core, prefs) { applied += prefs.backgroundSync }

    @Test
    fun theFirstFrameComesFromThePreferences() {
        val s = repo.state.value
        assertEquals("light", s.theme)
        assertEquals(30, s.graceSeconds)
        assertEquals(listOf(10, 30, 60), s.graceChoices)
        assertFalse(s.cloudReady)
    }

    @Test
    fun refreshTakesTheCoresSettingsAndRemembersTheTheme() = runBlocking {
        repo.refresh()
        val s = repo.state.value
        assertEquals("dark", s.theme)
        assertEquals("dark", prefs.theme)
        assertEquals("Phone", s.settings.deviceName)
        assertEquals(listOf("gdrive"), s.remotes.map { it.name })
    }

    @Test
    fun aFailedRefreshChangesNothing() = runBlocking {
        val failing = object : SettingsCore by core {
            override suspend fun remotes(): List<Remote> = throw IllegalStateException("no core")
        }
        val r = SettingsRepository(failing, prefs) {}
        try {
            r.refresh()
            fail("expected the core's error")
        } catch (e: IllegalStateException) {
            assertEquals("no core", e.message)
        }
        assertEquals("light", prefs.theme)
        assertEquals("light", r.state.value.theme)
    }

    @Test
    fun theThemeAppliesAtOnceAndIsKeptInTheCore() = runBlocking {
        repo.setTheme("dark")
        assertEquals("dark", repo.state.value.theme)
        assertEquals("dark", prefs.theme)
        assertEquals(JsonPrimitive("dark"), core.updates.single()["theme"])
    }

    @Test
    fun aCoreThatFailsLeavesTheThemeChosenHere() = runBlocking {
        core.updateError = IllegalStateException("disk full")
        try {
            repo.setTheme("dark")
            fail("expected the core's error")
        } catch (e: IllegalStateException) {
            assertEquals("disk full", e.message)
        }
        assertEquals("dark", repo.state.value.theme)
        assertEquals("dark", prefs.theme)
    }

    @Test
    fun theDeviceNameIsTrimmed() = runBlocking {
        repo.setDeviceName("  Steam Deck ")
        assertEquals("Steam Deck", repo.state.value.settings.deviceName)
        assertEquals(JsonPrimitive("Steam Deck"), core.updates.single()["deviceName"])
    }

    @Test
    fun theWallpaperAndGraceChoicesArePersisted() {
        repo.setWallpaper(true)
        repo.setGraceSeconds(60)
        assertTrue(prefs.wallpaper)
        assertEquals(60, prefs.graceSeconds)
        assertTrue(repo.state.value.wallpaper)
        assertEquals(60, repo.state.value.graceSeconds)
    }

    @Test
    fun theBackgroundSyncIsPersistedBeforeItIsApplied() {
        repo.setBackgroundSync(true)
        assertEquals("the work is scheduled from the saved setting", listOf(true), applied)
        assertTrue(repo.state.value.backgroundSync)
        repo.setBackgroundSync(false)
        assertEquals(listOf(true, false), applied)
    }

    @Test
    fun connectingACloudTestsSavesAndSyncsTheLibrary() = runBlocking {
        val stale = mutableListOf<Unit>()
        val job = CoroutineScope(Dispatchers.Unconfined).launch { repo.libraryStale.collect { stale += it } }

        repo.useCloud("gdrive", "Carryover")

        assertEquals(listOf("test gdrive:Carryover", "update", "syncLibrary"), core.calls)
        val saved = core.updates.single()
        assertEquals(JsonPrimitive("gdrive"), saved["remote"])
        assertEquals(JsonPrimitive("Carryover"), saved["remoteRoot"])
        assertEquals(JsonPrimitive(true), saved["syncLibrary"])
        assertTrue(repo.state.value.cloudReady)
        assertEquals("the library is stale once it was synced", 1, stale.size)
        job.cancel()
    }

    @Test
    fun aCloudThatFailsTheTestIsNotSaved() = runBlocking {
        core.testError = IllegalStateException("login expired")
        try {
            repo.useCloud("gdrive", "Carryover")
            fail("expected the test's error")
        } catch (e: IllegalStateException) {
            assertEquals("login expired", e.message)
        }
        assertEquals(listOf("test gdrive:Carryover"), core.calls)
        assertFalse(repo.state.value.cloudReady)
    }

    @Test
    fun theKeyIsCheckedThenSavedTrimmed() = runBlocking {
        repo.setSteamGridKey("  abc123 ")
        assertEquals(listOf("testKey abc123", "update"), core.calls)
        assertEquals(JsonPrimitive("abc123"), core.updates.single()["steamGridDbKey"])
        assertTrue(repo.state.value.hasGridKey)
        assertEquals("abc123", repo.state.value.settings.steamGridDbKey)
    }

    @Test
    fun aKeySteamGridDBRejectsIsNotSaved() = runBlocking {
        core.keyError = IllegalStateException("SteamGridDB rejected the API key")
        try {
            repo.setSteamGridKey("bad")
            fail("expected the rejection")
        } catch (e: IllegalStateException) {
            assertEquals("SteamGridDB rejected the API key", e.message)
        }
        assertEquals(listOf("testKey bad"), core.calls)
        assertFalse(repo.state.value.hasGridKey)
    }

    @Test
    fun theLibraryIsSharedByDefaultOnlyOnceTheCoreSaysSo() = runBlocking {
        assertFalse(repo.state.value.sharesLibrary)
        assertTrue("covers follow the library unless kept local", repo.state.value.syncsCovers)
        repo.setShareLibrary(true)
        assertEquals(JsonPrimitive(true), core.updates.single()["syncLibrary"])
        assertTrue(repo.state.value.sharesLibrary)
        repo.setShareLibrary(false)
        assertFalse(repo.state.value.sharesLibrary)
        assertEquals("saving a setting does not merge anything", listOf("update", "update"), core.calls)
    }

    @Test
    fun coversAreKeptLocalWhenNotSynced() = runBlocking {
        repo.setSyncCovers(false)
        assertEquals(JsonPrimitive(true), core.updates.single()["coversLocalOnly"])
        assertFalse(repo.state.value.syncsCovers)
        repo.setSyncCovers(true)
        assertEquals(JsonPrimitive(false), core.updates.last()["coversLocalOnly"])
        assertTrue(repo.state.value.syncsCovers)
    }

    @Test
    fun refreshingTheLibraryMergesItAndMakesThePagesReadItAgain() = runBlocking {
        val stale = mutableListOf<Unit>()
        val job = CoroutineScope(Dispatchers.Unconfined).launch { repo.libraryStale.collect { stale += it } }
        repo.refreshLibrary()
        assertEquals(listOf("syncLibrary"), core.calls)
        assertEquals(1, stale.size)
        core.libraryError = IllegalStateException("offline")
        try {
            repo.refreshLibrary()
            fail("expected the error")
        } catch (e: IllegalStateException) {
            assertEquals("offline", e.message)
        }
        assertEquals("a failed merge is not a reason to read the library again", 1, stale.size)
        job.cancel()
    }

    @Test
    fun theBackupsKeptAreSavedAndShown() = runBlocking {
        assertEquals(0, repo.state.value.backupsKept)
        repo.setBackupsKept(7)
        assertEquals(JsonPrimitive(7), core.updates.single()["backupRetention"])
        assertEquals(7, repo.state.value.backupsKept)
        repo.setBackupsKept(0)
        assertEquals(0, repo.state.value.backupsKept)
    }

    @Test
    fun aNumberOfBackupsTheCoreWouldRefuseIsNotSent() = runBlocking {
        for (bad in listOf(-1, 101)) {
            try {
                repo.setBackupsKept(bad)
                fail("expected $bad to be refused")
            } catch (e: IllegalArgumentException) {
                // refused before the core is asked
            }
        }
        assertTrue(core.calls.isEmpty())
        repo.setBackupsKept(100)
        assertEquals(100, repo.state.value.backupsKept)
    }

    @Test
    fun anEmptyKeyRemovesItWithoutCheckingAnything() = runBlocking {
        repo.setSteamGridKey("abc")
        core.calls.clear()
        repo.setSteamGridKey("  ")
        assertEquals(listOf("update"), core.calls)
        assertFalse(repo.state.value.hasGridKey)
    }

    @Test
    fun noKeyUntilOneIsSet() {
        assertFalse(repo.state.value.hasGridKey)
    }

    @Test
    fun deletingAnAccountLeavesTheRefreshToTheCaller() = runBlocking {
        repo.deleteRemote("gdrive")
        assertEquals(listOf("delete gdrive"), core.calls)
        repo.refresh()
        assertEquals(emptyList<String>(), repo.state.value.remotes.map { it.name })
    }
}
