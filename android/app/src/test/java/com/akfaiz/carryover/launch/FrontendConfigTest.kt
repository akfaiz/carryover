package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.EmulatorPresets
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class FrontendConfigTest {
    private val carryover = "com.akfaiz.carryover/com.akfaiz.carryover.launch.LaunchActivity"

    private fun preset(name: String) = EmulatorPresets.all.first { it.name == name }

    @Test
    fun dataUriEmulator() {
        val p = preset("PPSSPP")
        val e = FrontendConfig.emulator("PPSSPP", EmulatorPresets.exeFor(p, p.packages[0]), p.template, null)
        assertEquals("org.ppsspp.ppsspp/org.ppsspp.ppsspp.PpssppActivity", e.target)
        assertEquals(
            """<command label="PPSSPP (Carryover)">%EMULATOR_CARRYOVER% %ACTION%=android.intent.action.VIEW """ +
                """%DATA%=%ROMSAF% %EXTRA_carryover.target%=org.ppsspp.ppsspp/org.ppsspp.ppsspp.PpssppActivity</command>""",
            FrontendConfig.esdeCommand(e),
        )
        assertEquals(
            """
            -n $carryover
            -a android.intent.action.VIEW
            -d {file.uri}
            -e carryover.target org.ppsspp.ppsspp/org.ppsspp.ppsspp.PpssppActivity
            """.trimIndent(),
            FrontendConfig.daijishoArgs(e, carryover),
        )
    }

    @Test
    fun pathInExtras() {
        // RetroArch takes the game as a path in an extra, next to its own extras.
        val p = preset("RetroArch")
        val e = FrontendConfig.emulator("RetroArch", EmulatorPresets.exeFor(p, "com.retroarch"), p.template, null)
        val cmd = FrontendConfig.esdeCommand(e)
        assertTrue(cmd, cmd.contains("%EXTRA_ROM%=%ROM%"))
        assertTrue(cmd, cmd.contains("%EXTRA_LIBRETRO%=/data/data/com.retroarch/cores/CORE_libretro_android.so"))
        val args = FrontendConfig.daijishoArgs(e, carryover)
        assertTrue(args, args.contains("-e ROM {file.path}"))
        assertTrue(args, args.endsWith("-e carryover.target com.retroarch/com.retroarch.browser.retroactivity.RetroActivityFuture"))
    }

    @Test
    fun withoutTemplate() {
        // An emulator set up to "open the app": a frontend still sends the game file to open.
        val e = FrontendConfig.emulator("Some Emu", "com.example.emu", "", "com.example.emu.MainActivity")
        assertEquals("com.example.emu/com.example.emu.MainActivity", e.target)
        assertEquals(
            "adb shell am start -n $carryover -a android.intent.action.VIEW -d '<game file URI>' " +
                "-e carryover.target com.example.emu/com.example.emu.MainActivity",
            FrontendConfig.adbCommand(e, carryover),
        )
        // Its launcher screen unknown: the package alone.
        assertEquals("com.example.emu", FrontendConfig.emulator("Some Emu", "com.example.emu", "", null).target)
    }

    @Test
    fun daijishoPlayer() {
        val p = preset("Eden")
        val e = FrontendConfig.emulator("Eden", EmulatorPresets.exeFor(p, p.packages[0]), p.template, null)
        assertEquals("Eden (Carryover)", FrontendConfig.daijishoName(e))

        // Daijisho's own shape: group 1 is the name its scrapers search for.
        val source = FrontendConfig.daijishoRegex(p.extensions)
        val regex = Regex(source)
        assertEquals("Zelda TotK [0100F2C0115B6000]", regex.find("Zelda TotK [0100F2C0115B6000].nsp")!!.groupValues[1])
        assertTrue("upper case too", regex.matches("GAME.XCI"))
        assertTrue("another system's file", !regex.matches("game.iso"))

        // Unknown file types (RetroArch's depend on the core): any file with an extension.
        val anySource = FrontendConfig.daijishoRegex(emptyList())
        val any = Regex(anySource)
        assertEquals("Super Mario World (USA)", any.find("Super Mario World (USA).sfc")!!.groupValues[1])

        // Daijisho refuses to save a regex that doesn't start with ^ and end with $.
        for (s in listOf(source, anySource)) assertTrue(s, s.startsWith("^") && s.endsWith("$"))
    }

    @Test
    fun presetExtensionsNeedNoEscaping() {
        for (p in EmulatorPresets.all) {
            for (x in p.extensions) assertTrue("${p.name}: $x", x.matches(Regex("[a-z0-9]+")))
        }
    }

    @Test
    fun esdeFindRulesNameCarryover() {
        val rules = FrontendConfig.esdeFindRules(carryover)
        assertTrue(rules.contains("""<emulator name="CARRYOVER">"""))
        assertTrue(rules.contains("<entry>$carryover</entry>"))
    }

    @Test
    fun namesAreEscapedInXml() {
        val e = FrontendConfig.emulator("Azahar & Citra", "org.azahar_emu.azahar", "", null)
        assertTrue(FrontendConfig.esdeCommand(e).startsWith("""<command label="Azahar &amp; Citra (Carryover)">"""))
    }
}
