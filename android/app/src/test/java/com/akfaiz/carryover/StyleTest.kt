package com.akfaiz.carryover

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File

/**
 * The Android twin of the desktop's TestPagesUseTheKitStyle: screens build on the app's own
 * kit (`ui/Kit.kt`, with the look in `ui/Theme.kt`) instead of Material's raw pieces and
 * numbers of their own, so every page looks like the others and like the desktop.
 *
 * A line that has a reason to break a rule says `style:ok` in a comment.
 */
class StyleTest {
    private val src = File("src/main/java/com/akfaiz/carryover")

    /** Where the rules are kept: they may use what they ban. */
    private val kit = setOf("Kit.kt", "Theme.kt")

    private class Rule(val why: String, val pattern: Regex, val except: Set<String> = emptySet())

    private val rules = listOf(
        Rule("a corner radius of its own: use MaterialTheme.shapes", Regex("""RoundedCornerShape\(""")),
        Rule("a bare Card: use SectionCard (or SubCard)", Regex("""(?<![\w.])Card\(""")),
        Rule("a colour of its own: use the theme's (or a Tone)", Regex("""Color\(0x|Color\.Black|Color\.White""")),
        Rule("a filled icon: use the outlined one (the selected tab is the one exception)", Regex("""Icons\.(AutoMirrored\.)?Filled\."""), except = setOf("App.kt")),
        Rule(
            "a Material button: use AppButton",
            Regex("""(?<![\w.])(Button|OutlinedButton|TextButton|FilledTonalButton|ElevatedButton)\("""),
        ),
        Rule("a top bar of its own: use PageTopBar", Regex("""(?<![\w.])TopAppBar\(""")),
        Rule("a spinner of its own size: use Spinner", Regex("""CircularProgressIndicator\(""")),
    )

    @Test
    fun screensUseTheKit() {
        val found = listOf("ui", "launch").flatMap { dir ->
            File(src, dir).walk().filter { it.extension == "kt" && it.name !in kit }.flatMap { f ->
                f.readLines().mapIndexedNotNull { i, line ->
                    val code = line.trim()
                    if (code.startsWith("//") || code.startsWith("*") || code.startsWith("/*") || "style:ok" in line) return@mapIndexedNotNull null
                    // Imports are not uses.
                    if (code.startsWith("import ")) return@mapIndexedNotNull null
                    rules.firstOrNull { f.name !in it.except && it.pattern.containsMatchIn(line) }?.let { "${f.name}:${i + 1}: ${it.why}: $code" }
                }
            }.toList()
        }
        assertEquals("screens that go around the kit", emptyList<String>(), found)
    }

    @Test
    fun theKitStaysOutOfTheScreens() {
        // Kit.kt is the one place Material's raw pieces are wrapped: it must not reach for a screen's state.
        val text = File(src, "ui/Kit.kt").readText()
        val bad = listOf("AppViewModel", "UiState", "Nav\\b", "Core\\b").filter { Regex(it).containsMatchIn(text) }
        assertEquals("Kit.kt knows about screens", emptyList<String>(), bad)
    }
}
