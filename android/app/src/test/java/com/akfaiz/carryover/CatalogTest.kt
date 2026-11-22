package com.akfaiz.carryover

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.w3c.dom.Element
import java.io.File
import javax.xml.parsers.DocumentBuilderFactory

/**
 * The app's text catalogs (like the desktop's TestLanguageCatalogsMatchEnglish
 * and TestTranslationCatalog): every language has every English text with the
 * same placeholders, and the screens take their text from the catalog.
 */
class CatalogTest {
    private val res = File("src/main/res")
    private val src = File("src/main/java/com/akfaiz/carryover")

    /** The translatable texts of a catalog by name ("plural:name" for plurals), with their placeholders. */
    private fun catalog(dir: String): Map<String, Set<String>> {
        val file = File(res, "$dir/strings.xml")
        assertTrue("$file is missing", file.isFile)
        val doc = DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(file)
        val out = mutableMapOf<String, Set<String>>()
        val root = doc.documentElement
        for (i in 0 until root.childNodes.length) {
            val e = root.childNodes.item(i) as? Element ?: continue
            if (e.getAttribute("translatable") == "false") continue
            val name = e.getAttribute("name")
            when (e.tagName) {
                "string" -> out[name] = placeholders(e.textContent)
                "plurals" -> {
                    val items = e.getElementsByTagName("item")
                    // Every form of a plural takes the same arguments; "other" is the one every language has.
                    val other = (0 until items.length).map { items.item(it) as Element }.first { it.getAttribute("quantity") == "other" }
                    out["plural:$name"] = placeholders(other.textContent)
                }
            }
        }
        return out
    }

    private fun placeholders(text: String): Set<String> = Regex("""%(\d+\$)?[sdf]""").findAll(text).map { it.value }.toSet()

    @Test
    fun indonesianMatchesEnglish() {
        val en = catalog("values")
        val id = catalog("values-in")
        assertEquals("texts missing in Bahasa Indonesia", emptySet<String>(), en.keys - id.keys)
        assertEquals("texts only in Bahasa Indonesia", emptySet<String>(), id.keys - en.keys)
        for ((name, ph) in en) {
            assertEquals("placeholders of $name", ph, id[name])
        }
    }

    @Test
    fun placeholdersArePositional() {
        // With several arguments a translation may reorder them; only %1$s and the like allow that.
        for ((name, ph) in catalog("values")) {
            if (ph.size > 1) assertTrue("$name: use %1\$s, %2\$s…", ph.all { it.contains('$') })
        }
    }

    @Test
    fun screensUseTheCatalog() {
        // A literal with a word in it, given to Text() or as a label, is English the catalog misses.
        val literal = Regex("""\bText\(\s*"[^"]*[A-Za-z]{2}""")
        val found = listOf("ui", "launch").flatMap { dir ->
            File(src, dir).walk().filter { it.extension == "kt" }.flatMap { f ->
                f.readLines().mapIndexedNotNull { i, line ->
                    if (literal.containsMatchIn(line)) "${f.name}:${i + 1}: ${line.trim()}" else null
                }
            }
        }
        assertEquals("hard-coded text", emptyList<String>(), found)
    }
}
