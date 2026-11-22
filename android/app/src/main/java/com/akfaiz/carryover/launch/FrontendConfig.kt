package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.Launch
import com.akfaiz.carryover.core.LaunchSpec

/**
 * The settings a frontend needs to start a game through Carryover (shown in
 * the app's Frontends page, docs/android-frontends.md): the launch the
 * frontend would send the emulator, sent to [LaunchActivity] instead, plus
 * `carryover.target` naming the emulator's screen. Built from the emulator's
 * start template (`am start` syntax with {rom}), so the game file goes where
 * that emulator expects it: as the data URI or as a path in an extra. Pure, so
 * it is unit tested.
 */
object FrontendConfig {
    /** What a frontend sends when an emulator has no start template: open this file. */
    const val DEFAULT_TEMPLATE = "-a android.intent.action.VIEW -d {rom}"

    /** A frontend's launch of one emulator, with the game file still as {rom}. */
    data class Emulator(
        /** Its name, for labels ("PPSSPP"). */
        val name: String,
        /** The screen to start: "package/full.class.Name", or just "package". */
        val target: String,
        val spec: LaunchSpec,
    )

    /**
     * The launch for an emulator: exe is its app ("package" or "package/activity"),
     * template its start template ("" uses [DEFAULT_TEMPLATE]), launcherActivity the
     * app's launcher screen when exe names none (from PackageManager; null if unknown).
     */
    fun emulator(name: String, exe: String, template: String, launcherActivity: String?): Emulator {
        val spec = Launch.parse(exe, template.ifBlank { DEFAULT_TEMPLATE }, Launch.ROM)
        val activity = spec.className ?: launcherActivity
        return Emulator(name, if (activity != null) "${spec.pkg}/$activity" else spec.pkg, spec)
    }

    // ES-DE

    /** custom_systems/es_find_rules.xml: Carryover as an emulator ES-DE can start. */
    fun esdeFindRules(carryover: String): String = """
        <ruleList>
            <emulator name="CARRYOVER">
                <!-- Carryover: syncs the saves, then starts the emulator named by carryover.target -->
                <rule type="androidpackage">
                    <entry>$carryover</entry>
                </rule>
            </emulator>
        </ruleList>
    """.trimIndent()

    /**
     * A <command> for custom_systems/es_systems.xml. The game file is %ROMSAF% (a
     * URI ES-DE lets the emulator read) as the data, %ROM% (a path) in an extra.
     */
    fun esdeCommand(e: Emulator): String {
        val s = e.spec
        val parts = mutableListOf("%EMULATOR_CARRYOVER%")
        s.action?.let { parts += "%ACTION%=$it" }
        s.data?.let { parts += "%DATA%=" + it.replace(Launch.ROM, "%ROMSAF%") }
        s.categories.forEach { parts += "%CATEGORY%=$it" }
        for (x in s.extras) {
            when (x) {
                is LaunchSpec.StringExtra -> parts += "%EXTRA_${x.key}%=" + x.value.replace(Launch.ROM, "%ROM%")
                is LaunchSpec.BoolExtra -> parts += "%EXTRABOOL_${x.key}%=${x.value}"
                // ES-DE has no number extras: the emulator gets its default.
                is LaunchSpec.IntExtra, is LaunchSpec.LongExtra -> {}
            }
        }
        parts += "%EXTRA_${Forward.EXTRA_TARGET}%=${e.target}"
        return """<command label="${xml(e.name)} (Carryover)">${xml(parts.joinToString(" "))}</command>"""
    }

    // Daijisho: a player has a name, am start arguments and a regex of the files it plays.

    /** The player's name. */
    fun daijishoName(e: Emulator): String = "${e.name} (Carryover)"

    /** The player's start arguments: {file.uri} as the data, {file.path} in an extra. */
    fun daijishoArgs(e: Emulator, carryover: String): String =
        amArgs(e, carryover, data = "{file.uri}", path = "{file.path}").joinToString("\n")

    /**
     * The player's accepted filename regex, in Daijisho's own shape (group 1 is the
     * name its scrapers search for): the emulator's file types, or any file with an
     * extension when they aren't known. Daijisho only takes a regex that starts with
     * ^ and ends with $, so instead of a case flag each type is listed in lower and
     * upper case (nsp|NSP).
     */
    fun daijishoRegex(extensions: List<String>): String =
        if (extensions.isEmpty()) {
            "^(.*)\\.[^.]+$"
        } else {
            // Extensions are letters and digits: nothing to escape.
            val types = extensions.flatMap { listOf(it.lowercase(), it.uppercase()) }.distinct()
            "^(.*)\\.(?:" + types.joinToString("|") + ")$"
        }

    // Any app

    /** The same launch from a computer, for trying it or for other launchers. */
    fun adbCommand(e: Emulator, carryover: String): String =
        "adb shell am start " + amArgs(e, carryover, data = "'<game file URI>'", path = "'<game file path>'").joinToString(" ")

    /** `am start` arguments that send e's launch to Carryover. */
    private fun amArgs(e: Emulator, carryover: String, data: String, path: String): List<String> = buildList {
        val s = e.spec
        add("-n $carryover")
        s.action?.let { add("-a $it") }
        s.data?.let { add("-d " + it.replace(Launch.ROM, data)) }
        s.type?.let { add("-t $it") }
        s.categories.forEach { add("-c $it") }
        for (x in s.extras) {
            add(
                when (x) {
                    is LaunchSpec.StringExtra -> "-e ${x.key} " + quote(x.value.replace(Launch.ROM, path))
                    is LaunchSpec.BoolExtra -> "--ez ${x.key} ${x.value}"
                    is LaunchSpec.IntExtra -> "--ei ${x.key} ${x.value}"
                    is LaunchSpec.LongExtra -> "--el ${x.key} ${x.value}"
                },
            )
        }
        add("-e ${Forward.EXTRA_TARGET} ${e.target}")
    }

    private fun quote(v: String) = if (v.any { it.isWhitespace() }) "\"$v\"" else v

    private fun xml(s: String) = s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;")
}
