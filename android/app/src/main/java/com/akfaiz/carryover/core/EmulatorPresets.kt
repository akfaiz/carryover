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
        ),
        EmulatorPreset(
            "Dolphin",
            listOf("org.dolphinemu.dolphinemu", "org.dolphinemu.mmjr"),
            activity = ".ui.main.MainActivity",
            template = "-a android.intent.action.VIEW -e AutoStartFile {rom}",
            manifestPreset = "dolphin",
            extensions = listOf("iso", "gcm", "gcz", "ciso", "wbfs", "rvz", "wia", "wad", "dol", "elf"),
        ),
        EmulatorPreset(
            "PPSSPP",
            listOf("org.ppsspp.ppsspp", "org.ppsspp.ppssppgold"),
            activity = ".PpssppActivity",
            template = "-a android.intent.action.VIEW -d {rom}",
            manifestPreset = "ppsspp",
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
            extensions = listOf("3ds", "3dsx", "cci", "cxi", "cia", "app", "elf", "axf"),
        ),
        EmulatorPreset(
            "NetherSX2 / AetherSX2",
            listOf("xyz.aethersx2.android"),
            activity = ".EmulationActivity",
            template = "-e bootPath {rom}",
            manifestPreset = "pcsx2",
            extensions = listOf("iso", "chd", "cso", "bin", "img", "mdf", "nrg", "gz", "elf"),
        ),
        EmulatorPreset(
            "DuckStation",
            listOf("com.github.stenzek.duckstation"),
            activity = ".EmulationActivity",
            template = "-e bootPath {rom}",
            manifestPreset = "duckstation",
            extensions = listOf("cue", "bin", "chd", "iso", "img", "ecm", "mds", "pbp", "m3u", "exe"),
        ),
        EmulatorPreset(
            "melonDS",
            listOf("me.magnum.melonds"),
            activity = ".ui.emulator.EmulatorActivity",
            template = "-e uri {rom}",
            extensions = listOf("nds", "dsi", "ids", "srl", "zip"),
        ),
        // No known way to start a game directly: the emulator opens, the user picks the game.
        EmulatorPreset("ScummVM", listOf("org.scummvm.scummvm"), manifestPreset = "scummvm"),
        EmulatorPreset("Vita3K", listOf("org.vita3k.emulator"), manifestPreset = "vita3k", extensions = listOf("vpk", "zip")),
    )

    /**
     * Whether a library emulator can be used on Android: it is set up on this
     * device, or it is a kind with an Android app (by the desktop's preset, or
     * by name for one without a preset). Ryujinx, Cemu, RPCS3, shadPS4 and
     * Xenia have none, so the app hides them and their games.
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

    /** The preset of an installed package, if it is a known emulator. */
    fun forPackage(pkg: String): EmulatorPreset? = all.firstOrNull { pkg in it.packages }

    /** The exe for a package of a preset: "package/activity" when the template needs that activity. */
    fun exeFor(preset: EmulatorPreset?, pkg: String): String =
        if (preset?.activity != null) "$pkg/${preset.activity}" else pkg
}
