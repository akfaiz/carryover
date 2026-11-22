package com.akfaiz.carryover.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SafPathTest {
    private val storage = "com.android.externalstorage.documents"

    @Test
    fun formatAndParse() {
        val p = SafPath.of(storage, "primary:Emu/saves")
        assertEquals(":saf:com.android.externalstorage.documents/primary%3AEmu%2Fsaves", p)
        assertEquals(SafPath.Parts(storage, "primary:Emu/saves", emptyList()), SafPath.parse(p))
        assertEquals(
            SafPath.Parts(storage, "primary:Emu/saves", listOf("sub", "a.sav")),
            SafPath.parse("$p/sub//a.sav/"),
        )
        assertTrue(SafPath.isSaf(p))
        assertFalse(SafPath.isSaf("/storage/emulated/0/Emu"))
        assertNull(SafPath.parse("/storage/emulated/0/Emu"))
        assertNull(SafPath.parse(":saf:only-authority"))
    }

    @Test
    fun encodeRoundTrips() {
        for (s in listOf("primary:Emu/saves", "1234-ABCD:Games/Ünïcode 名前", "a%b+c d", "")) {
            val e = SafPath.encode(s)
            assertFalse("$e contains a slash", e.contains('/'))
            assertEquals(s, SafPath.decode(e))
        }
    }

    @Test
    fun describe() {
        assertEquals("Internal storage › Emu/saves", SafPath.describe(SafPath.of(storage, "primary:Emu/saves")))
        assertEquals("Internal storage › Emu/saves/sub", SafPath.describe(SafPath.of(storage, "primary:Emu/saves") + "/sub"))
        assertEquals("Internal storage", SafPath.describe(SafPath.of(storage, "primary:")))
        assertEquals("SD card 1234-ABCD › Games", SafPath.describe(SafPath.of(storage, "1234-ABCD:Games")))
        val emu = SafPath.of("org.dolphinemu.dolphinemu.user", "root:GC")
        assertEquals("Dolphin › GC", SafPath.describe(emu) { if (it == "org.dolphinemu.dolphinemu.user") "Dolphin" else null })
        assertEquals("org.dolphinemu.dolphinemu.user › GC", SafPath.describe(emu))
        assertEquals("/plain/path", SafPath.describe("/plain/path"))
    }
}
