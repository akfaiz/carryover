package com.akfaiz.carryover.core

import com.akfaiz.carryover.R

/**
 * A known Android emulator: its app(s), an optional template that starts a
 * game directly (am start syntax, see [Launch]), and the save-path preset of
 * the desktop app (emumanifest) for the same emulator family.
 *
 * The packages and templates follow ES-DE's Android find rules and have not
 * all been tried on a device yet (docs/android-architecture.md: to verify). Without
 * a template, or without a game file, Carryover opens the emulator and the
 * user picks the game there, which works with any emulator.
 */
data class EmulatorPreset(
    val name: String,
    val packages: List<String>,
    val activity: String? = null,
    val template: String = "",
    val manifestPreset: String = "",
    /** What the template needs edited before it works, as a string resource (0: nothing). */
    val note: Int = 0,
    /** Where its saves are and how to reach them from Carryover, as a string resource (0: nothing to say). */
    val savesNote: Int = 0,
    /** The game files it opens, by extension (empty: depends, any file); for a frontend's file filter. */
    val extensions: List<String> = emptyList(),
)

object EmulatorPresets {
    val all = listOf(
        EmulatorPreset(
            "RetroArch",
            listOf("com.retroarch", "com.retroarch.aarch64", "com.retroarch.ra32"),
            activity = "com.retroarch.browser.retroactivity.RetroActivityFuture",
            template = "-e ROM {rom} -e LIBRETRO /data/data/com.retroarch/cores/CORE_libretro_android.so " +
                "-e CONFIGFILE /storage/emulated/0/Android/data/com.retroarch/files/retroarch.cfg",
            manifestPreset = "retroarch",
            note = R.string.preset_note_retroarch,
            savesNote = R.string.saves_note_provider,
        ),
        EmulatorPreset(
            "Dolphin",
            listOf("org.dolphinemu.dolphinemu", "org.dolphinemu.mmjr"),
            activity = ".ui.main.MainActivity",
            template = "-a android.intent.action.VIEW -e AutoStartFile {rom}",
            manifestPreset = "dolphin",
            savesNote = R.string.saves_note_provider,
            extensions = listOf("iso", "gcm", "gcz", "ciso", "wbfs", "rvz", "wia", "wad", "dol", "elf"),
        ),
        EmulatorPreset(
            "PPSSPP",
            listOf("org.ppsspp.ppsspp", "org.ppsspp.ppssppgold"),
            activity = ".PpssppActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "ppsspp",
            savesNote = R.string.saves_note_ppsspp,
            extensions = listOf("iso", "cso", "chd", "pbp", "elf", "zip"),
        ),
        EmulatorPreset(
            "Eden",
            listOf("dev.eden.eden_emulator"),
            activity = "org.yuzu.yuzu_emu.activities.EmulationActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "switch",
            extensions = listOf("nsp", "xci", "nca", "nro", "nso"),
        ),
        // Android-only, built on Eden: same Switch saves, so it can sync with Eden on a PC
        // (set up the game's Eden emulator on the phone with the Lemon app).
        EmulatorPreset(
            "Lemon",
            listOf("dev.lemon.lemon_emulator", "dev.lemon.lemon_emulator.lite", "dev.legacy.lemon_emulator"),
            activity = "dev.lemon.lemon_emu.activities.EmulationActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "switch",
            note = R.string.preset_note_lemon,
            extensions = listOf("nsp", "xci", "nca", "nro", "nso"),
        ),
        EmulatorPreset(
            "yuzu / Citron",
            listOf("org.yuzu.yuzu_emu", "org.citron.citron_emu"),
            activity = "org.yuzu.yuzu_emu.activities.EmulationActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "switch",
            extensions = listOf("nsp", "xci", "nca", "nro", "nso"),
        ),
        EmulatorPreset(
            "Azahar / Citra",
            listOf("io.github.lime3ds.android", "org.azahar_emu.azahar", "org.citra.citra_emu"),
            activity = "org.citra.citra_emu.activities.EmulationActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "azahar",
            savesNote = R.string.saves_note_picked,
            extensions = listOf("3ds", "3dsx", "cci", "cxi", "cia", "app", "elf", "axf"),
        ),
        EmulatorPreset(
            "melonDS",
            listOf("me.magnum.melonds"),
            activity = ".ui.emulator.EmulatorActivity",
            template = "-e uri {rom}",
            extensions = listOf("nds", "dsi", "ids", "srl", "zip"),
        ),
        // No known way to start a game directly: the emulator opens, the user picks the game.
        EmulatorPreset(
            "ScummVM",
            listOf("org.scummvm.scummvm"),
            manifestPreset = "scummvm",
            savesNote = R.string.saves_note_scummvm,
        ),
        EmulatorPreset(
            "Vita3K",
            listOf("org.vita3k.emulator"),
            manifestPreset = "vita3k",
            savesNote = R.string.saves_note_provider,
            extensions = listOf("vpk", "zip"),
        ),
    )

    /**
     * Whether a library emulator can be used on Android: it is set up on this
     * device, or it is a kind with an Android app (by the desktop's preset, or
     * by name for one without a preset). Ryujinx, Cemu, RPCS3, shadPS4 and
     * Xenia have no Android app, so the app hides them and their games.
     *
     * An emulator is listed only if its saves can be reached from here: it
     * publishes a documents provider that the folder picker shows (Eden,
     * Dolphin, RetroArch, Vita3K), or keeps them in a folder the user picks
     * (PPSSPP, Azahar, melonDS, ScummVM). NetherSX2/AetherSX2 and DuckStation
     * keep them in `Android/data` with no provider, which scoped storage hides
     * from every other app, so they are not listed (and PCSX2 / DuckStation
     * games are hidden), unless one is already set up on this device.
     */
    fun onAndroid(e: EmulatorView): Boolean {
        if (e.setUp || e.device.exe.isNotBlank()) return true
        if (e.preset.isNotEmpty()) return all.any { it.manifestPreset == e.preset }
        val name = normalize(e.name)
        return name.isNotEmpty() && all.any { p ->
            p.name.split('/').map(::normalize).any { it.isNotEmpty() && name.startsWith(it) }
        }
    }

    /** Lower-case letters and digits only: "Azahar / Citra" -> "azaharcitra". */
    private fun normalize(s: String): String = s.lowercase().filter { it.isLetterOrDigit() }

    /**
     * The note on reaching a library emulator's saves (a string resource, 0: none): its app's preset on
     * this device, else the first preset of its kind.
     */
    fun savesNote(e: EmulatorView?): Int {
        if (e == null) return 0
        val preset = forPackage(e.device.exe.substringBefore('/'))
            ?: all.firstOrNull { e.preset.isNotEmpty() && it.manifestPreset == e.preset && it.savesNote != 0 }
        return preset?.savesNote ?: 0
    }

    /** The preset of an installed package, if it is a known emulator. */
    fun forPackage(pkg: String): EmulatorPreset? = all.firstOrNull { pkg in it.packages }

    /** The exe for a package of a preset: "package/activity" when the template needs that activity. */
    fun exeFor(preset: EmulatorPreset?, pkg: String): String =
        if (preset?.activity != null) "$pkg/${preset.activity}" else pkg
}
