package com.akfaiz.carryover

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File

/**
 * The app's layers, like the desktop's depguard rules: the screens know a view
 * model, not the Go core, the preferences or the session machinery. The core
 * is reached through a [com.akfaiz.carryover.core.CoreApi] a view model holds,
 * so it can be replaced; and the session rules (`SessionManager`, `PlayFlow`)
 * have no Android classes, so a unit test can run them.
 */
class LayeringTest {
    private val src = File("src/main/java/com/akfaiz/carryover")

    private fun files(dir: String) = File(src, dir).walk().filter { it.extension == "kt" }.toList()

    /** The lines of [files] that match [pattern], as "File.kt:line: text". */
    private fun hits(files: List<File>, pattern: Regex): List<String> = files.flatMap { f ->
        f.readLines().mapIndexedNotNull { i, line ->
            if (pattern.containsMatchIn(line)) "${f.name}:${i + 1}: ${line.trim()}" else null
        }
    }

    @Test
    fun screensDoNotReachTheCoreOrThePreferences() {
        // A view model (AppViewModel) is where the app's state meets the core.
        val screens = files("ui").filterNot { it.name.endsWith("ViewModel.kt") }
        val forbidden = Regex("""\b(Core|AppPrefs|AppServices|AppPlatform|AndroidPlatform|SettingsRepository|PreferenceStore|Messenger|SessionManager|SessionService|BackgroundSync|SafStorage)\b""")
        assertEquals("screens must go through a view model", emptyList<String>(), hits(screens, forbidden))
    }

    @Test
    fun sessionRulesUseNoAndroidClasses() {
        // These run in plain JVM unit tests (SessionManagerTest, PlayFlowTest, PendingUploadsTest).
        val pure = listOf("session/SessionManager.kt", "session/PlayFlow.kt", "session/PendingUploads.kt", "session/Front.kt", "data/SettingsRepository.kt", "data/SyncController.kt")
            .map { File(src, it) }
        assertEquals("no android.* in the session rules", emptyList<String>(), hits(pure, Regex("""import (android|androidx)\.""")))
    }

    @Test
    fun viewModelsUseNoAndroidClasses() {
        // They run in plain JVM unit tests (AppViewModelTest, LaunchViewModelTest, ...): what they need of
        // Android comes through AppPlatform / LaunchPlatform and Texts.
        val models = (files("ui") + files("launch")).filter { it.name.endsWith("ViewModel.kt") }
        assertEquals("8 view models", 8, models.size)
        assertEquals("no android.* in a view model", emptyList<String>(), hits(models, Regex("""import android\.""")))
    }

    @Test
    fun uiFoldersImportOnlyDownward() {
        // The shell (the files in ui/ itself) may use every folder. A feature folder (game, settings, ...)
        // uses the shared ones and itself, never another feature or the shell: pages move with Nav.
        val shared = mapOf("kit" to emptySet(), "nav" to emptySet(), "state" to emptySet(), "components" to setOf("kit", "state"))
        val forAFeature = setOf("kit", "nav", "state", "components")
        val bad = File(src, "ui").listFiles().orEmpty().filter { it.isDirectory }.flatMap { dir ->
            val allowed = (shared[dir.name] ?: forAFeature) + dir.name
            hits(files("ui/${dir.name}"), Regex("""import com\.akfaiz\.carryover\.ui\.(?!(${allowed.joinToString("|")})\.)"""))
        }
        assertEquals("a ui/ folder imports one it must not", emptyList<String>(), bad)
    }

    @Test
    fun theDataLayerKnowsNoScreens() {
        val bad = hits(files("data"), Regex("""import com\.akfaiz\.carryover\.(ui|launch)\."""))
        assertEquals("data/ must not import the screens", emptyList<String>(), bad)
    }

    @Test
    fun theCoreLayerKnowsNoScreensOrSessions() {
        val bad = hits(files("core"), Regex("""import com\.akfaiz\.carryover\.(ui|session|launch|data)\."""))
        assertEquals("core/ must not import upward", emptyList<String>(), bad)
    }
}
