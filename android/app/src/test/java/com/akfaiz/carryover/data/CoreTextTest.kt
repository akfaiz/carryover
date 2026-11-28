package com.akfaiz.carryover.data

import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.CoreException
import com.akfaiz.carryover.core.CoreMessage
import com.akfaiz.carryover.core.coreError
import com.akfaiz.carryover.core.json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Test

private object CatalogIds : Texts {
    override fun string(id: Int, vararg args: Any) = "$id:" + args.joinToString("|")
    override fun plural(id: Int, count: Int, vararg args: Any) = "p$id#$count:" + args.joinToString("|")
}

/** The core's messages in the app's language (JSON as mobile and services send it). */
class CoreTextTest {
    private fun message(text: String): CoreMessage = json.decodeFromString(text)

    @Test
    fun theValuesGoIntoTheResourceInItsOrder() {
        val m = message("""{"code":"sync.uploading","args":{"Count":2,"Slot":"saves"},"text":"Uploading 2 files (saves)"}""")
        assertEquals("p${R.plurals.core_sync_uploading}#2:2|saves", CatalogIds.core(m))
    }

    @Test
    fun aListIsTranslatedItemByItem() {
        val m = message(
            """{"code":"setup.missing","args":{"Items":[{"code":"need.game_exe","text":"game executable"},""" +
                """{"code":"need.setup","text":"setup on this device"}]},"text":"Missing game executable, setup on this device"}""",
        )
        assertEquals(
            "${R.string.core_setup_missing}:${R.string.core_need_game_exe}:, ${R.string.core_need_setup}:",
            CatalogIds.core(m),
        )
    }

    @Test
    fun theCauseFollowsAndKeepsOtherProgramsWords() {
        val m = message(
            """{"code":"session.upload_failed","cause":{"text":"quota exceeded"},""" +
                """"text":"Game closed, but upload failed: quota exceeded"}""",
        )
        assertEquals("${R.string.core_session_upload_failed}:: quota exceeded", CatalogIds.core(m))
        val led = message("""{"cause":{"code":"cloud.offline","text":"no internet connection"},"text":"step 2: no internet connection"}""")
        assertEquals("step 2: ${R.string.core_cloud_offline}:", CatalogIds.core(led))
    }

    @Test
    fun anUnknownCodeAndAMissingMessageStayInEnglish() {
        assertEquals("Something new", CatalogIds.core(message("""{"code":"new.one","text":"Something new"}""")))
        assertEquals("older core", CatalogIds.core("older core", null))
    }

    @Test
    fun anErrorOfTheCoreIsReadFromItsText() {
        val e = coreError(Exception("""x:{"code":"library.game_not_found","text":"game not found"}"""), "x:")
        assertEquals("game not found", e.message)
        assertEquals("library.game_not_found", (e as CoreException).detail.code)
        assertEquals("${R.string.core_library_game_not_found}:", CatalogIds.error(e))

        val plain = Exception("disk full")
        assertSame(plain, coreError(plain, "x:"))
        assertEquals("disk full", CatalogIds.error(plain))
        val broken = Exception("x:{not json")
        assertSame(broken, coreError(broken, "x:"))
    }
}
