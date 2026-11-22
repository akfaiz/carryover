package com.akfaiz.carryover.ui

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.ImportResult
import com.akfaiz.carryover.core.Provider
import com.akfaiz.carryover.core.Question
import com.akfaiz.carryover.core.Remote
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The settings and cloud pages. */
class SettingsAndCloudViewModelTest {
    private val h = Harness()

    @After
    fun tearDown() = h.close()

    private fun settings() = SettingsViewModel(h.services)
    private fun cloud() = CloudViewModel(h.services)

    private fun error(what: String, why: String) = h.text(R.string.msg_error, what, why)

    // Settings

    @Test
    fun theThemeIsSetAtOnceAndKeptInTheCore() {
        val vm = settings()
        vm.setTheme("dark")
        assertEquals("dark", vm.state.value.theme)
        assertEquals("dark", h.prefs.theme)
        assertEquals("dark", h.core.settingsCore.settings.theme)
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun aThemeTheCoreCantKeepIsSaidButStaysChosen() {
        h.core.settingsCore.updateError = IllegalStateException("disk full")
        val vm = settings()
        vm.setTheme("dark")
        assertEquals("dark", vm.state.value.theme)
        assertEquals(listOf(error(h.text(R.string.err_save), "disk full")), h.said)
    }

    @Test
    fun theDeviceNameIsTrimmedAndSavedWithAMessage() {
        val vm = settings()
        vm.setDeviceName("  Steam Deck ")
        assertEquals("Steam Deck", vm.state.value.settings.deviceName)
        assertEquals(listOf(h.text(R.string.msg_saved)), h.said)
    }

    @Test
    fun aDeviceNameTheCoreRefusesIsSaid() {
        h.core.settingsCore.updateError = IllegalStateException("no")
        settings().setDeviceName("X")
        assertEquals(listOf(error(h.text(R.string.err_save), "no")), h.said)
    }

    @Test
    fun thePlaySessionPreferencesAreKept() {
        val vm = settings()
        vm.setGraceSeconds(60)
        vm.setWallpaper(true)
        vm.setBackgroundSync(true)
        assertEquals(60, h.prefs.graceSeconds)
        assertTrue(h.prefs.wallpaper)
        assertTrue(h.prefs.backgroundSync)
        assertEquals(60, vm.state.value.graceSeconds)
        assertEquals(listOf(10, 30, 60), vm.state.value.graceChoices)
    }

    @Test
    fun aKeyIsCheckedSavedAndSaid() {
        var saved: Boolean? = null
        settings().saveSteamGridKey("abc") { saved = it }
        assertEquals(true, saved)
        assertTrue("testKey abc" in h.core.settingsCore.calls)
        assertEquals(listOf(h.text(R.string.msg_grid_key_saved)), h.said)
        assertTrue(settings().state.value.hasGridKey)
    }

    @Test
    fun aKeyThatIsRejectedIsSaidAndNotSaved() {
        h.core.settingsCore.keyError = IllegalStateException("SteamGridDB rejected the API key")
        var saved: Boolean? = null
        val vm = settings()
        vm.saveSteamGridKey("bad") { saved = it }
        assertEquals(false, saved)
        assertEquals(listOf(error(h.text(R.string.err_grid_key), "SteamGridDB rejected the API key")), h.said)
        assertEquals(false, vm.state.value.hasGridKey)
    }

    @Test
    fun theKeyIsRemovedWithAnEmptyOne() {
        val vm = settings()
        vm.saveSteamGridKey("abc") {}
        h.said.clear()
        vm.saveSteamGridKey("") {}
        assertEquals(listOf(h.text(R.string.msg_grid_key_removed)), h.said)
        assertEquals(false, vm.state.value.hasGridKey)
    }

    @Test
    fun sharingTheLibrarySavesItAndMergesIt() {
        val vm = settings()
        vm.setShareLibrary(true)
        assertEquals(listOf("update", "syncLibrary"), h.core.settingsCore.calls.filter { it != "settings" })
        assertTrue(vm.state.value.sharesLibrary)
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun notSharingTheLibraryOnlySavesIt() {
        val vm = settings()
        vm.setShareLibrary(true)
        h.core.settingsCore.calls.clear()
        vm.setShareLibrary(false)
        assertEquals(listOf("update"), h.core.settingsCore.calls)
        assertFalse(vm.state.value.sharesLibrary)
    }

    @Test
    fun aFailedMergeIsSaidAndTheSettingStays() {
        val vm = settings()
        h.core.settingsCore.libraryError = IllegalStateException("offline")
        vm.setShareLibrary(true)
        assertTrue(vm.state.value.sharesLibrary)
        assertEquals(listOf(error(h.text(R.string.err_library_refresh), "offline")), h.said)
    }

    @Test
    fun aSettingTheCoreRefusesIsSaidAndNothingIsMerged() {
        val vm = settings()
        h.core.settingsCore.updateError = IllegalStateException("disk full")
        vm.setShareLibrary(true)
        assertFalse(vm.state.value.sharesLibrary)
        assertEquals(listOf(error(h.text(R.string.err_library_setting), "disk full")), h.said)
        assertTrue("syncLibrary" !in h.core.settingsCore.calls)
    }

    @Test
    fun coversAreFetchedWhenTurnedOnAndLeftAloneWhenTurnedOff() {
        val vm = settings()
        vm.setSyncCovers(false)
        assertFalse(vm.state.value.syncsCovers)
        assertTrue("syncLibrary" !in h.core.settingsCore.calls)
        vm.setSyncCovers(true)
        assertTrue(vm.state.value.syncsCovers)
        assertEquals(1, h.core.settingsCore.calls.count { it == "syncLibrary" })
    }

    @Test
    fun theBackupsKeptAreSavedAndSaid() {
        val vm = settings()
        vm.setBackupsKept(5)
        assertEquals(5, vm.state.value.backupsKept)
        assertEquals(listOf(h.plural(R.plurals.msg_backups_on, 5, 5)), h.said)
        h.said.clear()
        vm.setBackupsKept(0)
        assertEquals(0, vm.state.value.backupsKept)
        assertEquals(listOf(h.text(R.string.msg_backups_off)), h.said)
    }

    @Test
    fun aBackupSettingTheCoreRefusesIsSaid() {
        val vm = settings()
        h.core.settingsCore.updateError = IllegalStateException("disk full")
        vm.setBackupsKept(3)
        assertEquals(listOf(error(h.text(R.string.err_backups_setting), "disk full")), h.said)
        assertEquals(0, vm.state.value.backupsKept)
    }

    @Test
    fun theKeyPageOpensInTheBrowser() {
        settings().openBrowser("https://www.steamgriddb.com/profile/preferences/api")
        assertTrue("openBrowser https://www.steamgriddb.com/profile/preferences/api" in h.core.calls)
    }

    @Test
    fun theCoreVersionIsShown() {
        assertEquals("test-core", settings().coreVersion)
    }

    @Test
    fun theLogsAreCollectedIntoAZip() = runBlocking {
        val zip = File.createTempFile("logs", ".zip")
        h.platform.logs = zip
        assertEquals(zip, settings().collectLogs())
        assertTrue(h.said.isEmpty())
    }

    @Test
    fun logsThatCantBeCollectedAreSaid() = runBlocking {
        h.platform.logsError = IllegalStateException("no space")
        assertNull(settings().collectLogs())
        assertEquals(listOf(error(h.text(R.string.err_logs), "no space")), h.said)
    }

    @Test
    fun aZipIsSavedToTheDocumentThePickedAndSaid() {
        val zip = File.createTempFile("logs", ".zip")
        settings().saveLogs(zip, "content://docs/1")
        assertEquals(listOf(zip to "content://docs/1"), h.platform.savedLogs)
        assertEquals(listOf(h.text(R.string.msg_logs_saved)), h.said)
    }

    @Test
    fun aZipThatCantBeSavedIsSaid() {
        h.platform.saveLogsError = IllegalStateException("read-only")
        settings().saveLogs(File.createTempFile("logs", ".zip"), "content://docs/1")
        assertEquals(listOf(error(h.text(R.string.err_logs), "read-only")), h.said)
    }

    // Cloud

    @Test
    fun theAccountsAreReadFromTheCore() {
        val vm = cloud()
        vm.refresh()
        assertEquals(listOf("gdrive"), vm.state.value.remotes.map { it.name })
    }

    @Test
    fun accountsThatCantBeReadAreSaid() {
        h.core.settingsCore.settingsError = IllegalStateException("no core")
        cloud().refresh()
        assertEquals(listOf(error(h.text(R.string.err_read_settings), "no core")), h.said)
    }

    @Test
    fun aCloudIsTestedSavedAndSaid() {
        val vm = cloud()
        var ok: Boolean? = null
        vm.useCloud("gdrive", "Carryover") { ok = it }
        assertEquals(listOf("test gdrive:Carryover", "update", "syncLibrary"), h.core.settingsCore.calls)
        assertTrue(vm.state.value.cloudReady)
        assertEquals(listOf(h.text(R.string.msg_cloud_connected)), h.said)
        assertEquals("the page may close", true, ok)
    }

    @Test
    fun aCloudThatFailsTheTestIsSaidAndNotSaved() {
        h.core.settingsCore.testError = IllegalStateException("login expired")
        val vm = cloud()
        var ok: Boolean? = null
        vm.useCloud("gdrive", "Carryover") { ok = it }
        assertEquals(listOf(error(h.text(R.string.err_use_cloud, "gdrive:Carryover"), "login expired")), h.said)
        assertEquals(false, vm.state.value.cloudReady)
        assertEquals("the page stays open", false, ok)
    }

    @Test
    fun deletingAnAccountRefreshesTheListAndSaysSo() {
        val vm = cloud()
        vm.refresh()
        vm.deleteRemote("gdrive")
        assertTrue("delete gdrive" in h.core.settingsCore.calls)
        assertEquals(emptyList<String>(), vm.state.value.remotes.map { it.name })
        assertEquals(listOf(h.text(R.string.msg_remote_deleted, "gdrive")), h.said)
    }

    @Test
    fun aFailedDeleteIsSaidAndTheListIsStillRefreshed() {
        val vm = cloud()
        vm.refresh()
        val reads = h.core.settingsCore.calls.count { it == "settings" }
        h.core.settingsCore.deleteError = IllegalStateException("in use")
        vm.deleteRemote("gdrive")
        assertEquals(listOf(error(h.text(R.string.err_delete_remote, "gdrive"), "in use")), h.said)
        assertEquals("read again anyway", reads + 1, h.core.settingsCore.calls.count { it == "settings" })
        assertEquals(listOf("gdrive"), vm.state.value.remotes.map { it.name })
    }

    @Test
    fun importingAnRcloneConfigSaysHowManyAndReadsTheAccountsAgain() {
        val vm = cloud()
        h.core.importResult = ImportResult(imported = listOf("gdrive", "nas"))
        val reads = h.core.settingsCore.calls.count { it == "settings" }
        vm.importConfig("[gdrive]\ntype = drive\n")
        assertEquals(listOf("importConfig [gdrive]\ntype = drive"), h.core.calls.filter { it.startsWith("importConfig") })
        assertEquals(listOf(h.plural(R.plurals.msg_import_done, 2, 2)), h.said)
        assertEquals("the new accounts show", reads + 1, h.core.settingsCore.calls.count { it == "settings" })
    }

    @Test
    fun anImportSaysWhatItReplacedAndWhatItLeftOutInOneMessage() {
        val vm = cloud()
        h.core.importResult = ImportResult(imported = listOf("nas"), skipped = listOf("bucket (s3)", "x (ftp)"), replaced = listOf("nas"))
        vm.importConfig("conf")
        assertEquals(
            listOf(
                listOf(
                    h.plural(R.plurals.msg_import_done, 1, 1),
                    h.text(R.string.msg_import_replaced, "nas"),
                    h.text(R.string.msg_import_skipped, "bucket (s3), x (ftp)"),
                ).joinToString(" "),
            ),
            h.said,
        )
    }

    @Test
    fun aFileWithNothingOfASupportedKindSaysSo() {
        val vm = cloud()
        h.core.importResult = ImportResult(skipped = listOf("bucket (s3)"))
        vm.importConfig("conf")
        assertEquals(listOf(h.text(R.string.msg_import_none) + " " + h.text(R.string.msg_import_skipped, "bucket (s3)")), h.said)
    }

    @Test
    fun aConfigTheCoreRefusesIsSaid() {
        val vm = cloud()
        h.core.importError = IllegalStateException("encrypted rclone configs are not supported")
        val reads = h.core.settingsCore.calls.count { it == "settings" }
        vm.importConfig("RCLONE_ENCRYPT_V0:")
        assertEquals(listOf(error(h.text(R.string.err_import_config), "encrypted rclone configs are not supported")), h.said)
        assertEquals("nothing changed, so nothing to read again", reads, h.core.settingsCore.calls.count { it == "settings" })
    }

    @Test
    fun aFileThatCouldNotBeReadIsSaid() {
        cloud().importUnreadable(java.io.IOException("no such file"))
        assertEquals(listOf(error(h.text(R.string.err_import_config), "no such file")), h.said)
        assertTrue(h.core.calls.none { it.startsWith("importConfig") })
    }

    @Test
    fun connectingAProviderGoesThroughTheCore() = runBlocking {
        h.core.providers = listOf(Provider(type = "drive", description = "Google Drive"))
        val vm = cloud()
        assertEquals(listOf("drive"), vm.providers().map { it.type })
        h.core.question = Question(done = false, state = "s1", name = "token")
        val q = vm.createRemote("gdrive", "drive", mapOf("scope" to "drive"))
        assertEquals("s1", q.state)
        vm.continueRemote("gdrive", "s1", "answer")
        assertTrue("createRemote gdrive|drive|scope=drive" in h.core.calls)
        assertTrue("continueRemote gdrive|s1|answer" in h.core.calls)
    }

    @Test
    fun givingUpOnTheBrowserSignInGoesToTheCore() {
        cloud().cancelSignIn()
        assertTrue("cancelSignIn" in h.core.calls)
    }

    @Test
    fun anAccountWhoseSignInNeverFinishedIsFlagged() {
        val remotes = kotlinx.serialization.json.Json { ignoreUnknownKeys = true }
            .decodeFromString<List<Remote>>("""[{"name":"a","type":"dropbox","supported":true,"incomplete":true},{"name":"b","type":"smb","supported":true}]""")
        assertTrue(remotes[0].incomplete)
        assertFalse("the field is left out when false", remotes[1].incomplete)
    }

    @Test
    fun theSignInPageIsKeptUntilCleared() {
        val vm = cloud()
        h.core.authUrl.value = "https://auth.example/x"
        assertEquals("https://auth.example/x", vm.authUrl.value)
        vm.openBrowser("https://auth.example/x")
        assertTrue("openBrowser https://auth.example/x" in h.core.calls)
        vm.clearAuthUrl()
        assertNull(vm.authUrl.value)
        assertNotNull(vm.state.value)
    }
}
