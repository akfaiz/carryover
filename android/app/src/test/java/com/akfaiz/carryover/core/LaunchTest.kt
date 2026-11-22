package com.akfaiz.carryover.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class LaunchTest {
    @Test
    fun packageAloneOpensTheApp() {
        val s = Launch.parse("org.ppsspp.ppsspp")
        assertEquals(LaunchSpec("org.ppsspp.ppsspp"), s)
        assertTrue(s.opensApp)
    }

    @Test
    fun withoutRomOrTemplateTheAppOpens() {
        val exe = "org.ppsspp.ppsspp/.PpssppActivity"
        val want = LaunchSpec("org.ppsspp.ppsspp", ".PpssppActivity")
        assertEquals(want, Launch.parse(exe, "-a android.intent.action.VIEW -d {rom}", ""))
        assertEquals(want, Launch.parse(exe, "", "/sdcard/game.iso"))
        assertEquals("org.ppsspp.ppsspp.PpssppActivity", want.className)
    }

    @Test
    fun templateWithRom() {
        val s = Launch.parse(
            "com.retroarch/com.retroarch.browser.retroactivity.RetroActivityFuture",
            "-e ROM {rom} --ez QUIT true --ei SLOT 2 --el BIG 9000000000 -c android.intent.category.DEFAULT",
            "/storage/emulated/0/ROMs/My Game.sfc",
        )
        assertEquals("com.retroarch.browser.retroactivity.RetroActivityFuture", s.className)
        assertNull(s.action)
        assertEquals(
            listOf(
                LaunchSpec.StringExtra("ROM", "/storage/emulated/0/ROMs/My Game.sfc"),
                LaunchSpec.BoolExtra("QUIT", true),
                LaunchSpec.IntExtra("SLOT", 2),
                LaunchSpec.LongExtra("BIG", 9000000000),
            ),
            s.extras,
        )
        assertEquals(listOf("android.intent.category.DEFAULT"), s.categories)
        assertTrue(s.contentUris.isEmpty())
    }

    @Test
    fun contentUrisAreGranted() {
        val rom = "content://com.android.externalstorage.documents/document/primary%3AROMs%2Fgame.iso"
        val s = Launch.parse("org.ppsspp.ppsspp/.PpssppActivity", "-a android.intent.action.VIEW -d {rom} -t application/octet-stream", rom)
        assertEquals("android.intent.action.VIEW", s.action)
        assertEquals(rom, s.data)
        assertEquals("application/octet-stream", s.type)
        assertEquals(listOf(rom), s.contentUris)
    }

    @Test
    fun componentFlagMustNameTheSameApp() {
        val s = Launch.parse("org.dolphinemu.dolphinemu", "-n org.dolphinemu.dolphinemu/.ui.main.MainActivity -e AutoStartFile {rom}", "/g.iso")
        assertEquals("org.dolphinemu.dolphinemu.ui.main.MainActivity", s.className)
        assertThrows(LaunchException::class.java) {
            Launch.parse("org.dolphinemu.dolphinemu", "-n com.other/.Main", "/g.iso")
        }
    }

    @Test
    fun badSetups() {
        assertThrows(LaunchException::class.java) { Launch.parse("") }
        assertThrows(LaunchException::class.java) { Launch.parse("not a package") }
        assertThrows(LaunchException::class.java) { Launch.parse("a.b", "-x y", "/g") }
        assertThrows(LaunchException::class.java) { Launch.parse("a.b", "-e KEY", "/g") }
        assertThrows(LaunchException::class.java) { Launch.parse("a.b", "--ez K maybe", "/g") }
        assertThrows(LaunchException::class.java) { Launch.parse("a.b", "--ei K x", "/g") }
        assertThrows(LaunchException::class.java) { Launch.parse("a.b", "-e K \"open", "/g") }
    }

    @Test
    fun tokenizeKeepsQuotedParts() {
        assertEquals(listOf("-e", "A B", "-d", "x'y", ""), Launch.tokenize("-e \"A B\"  -d \"x'y\" ''"))
    }

    @Test
    fun presetsParse() {
        for (p in EmulatorPresets.all) {
            for (pkg in p.packages) {
                val s = Launch.parse(EmulatorPresets.exeFor(p, pkg), p.template, "/storage/emulated/0/game.bin")
                assertEquals(pkg, s.pkg)
                // Without a template the emulator opens and the user picks the game there.
                if (p.template.isNotEmpty()) {
                    assertTrue("${p.name} has no game in its template", s.toString().contains("/storage/emulated/0/game.bin"))
                }
            }
        }
        assertEquals("PPSSPP", EmulatorPresets.forPackage("org.ppsspp.ppssppgold")?.name)
        assertNull(EmulatorPresets.forPackage("com.example"))
    }
}
