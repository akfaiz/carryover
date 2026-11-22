package com.akfaiz.carryover.core

import android.content.ClipData
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri

/**
 * How to start an app, parsed from an emulator's (or game's) setup on this
 * device: exe is "package" or "package/activity" (".X" is relative to the
 * package), args an optional template in `am start` syntax that starts a game
 * directly, with {rom} replaced by the game file. Pure data, so it is unit
 * tested without Android; [toIntent] builds the Intent.
 */
data class LaunchSpec(
    val pkg: String,
    val activity: String? = null,
    val action: String? = null,
    val data: String? = null,
    val type: String? = null,
    val categories: List<String> = emptyList(),
    val extras: List<Extra> = emptyList(),
) {
    sealed interface Extra {
        val key: String
    }

    data class StringExtra(override val key: String, val value: String) : Extra
    data class BoolExtra(override val key: String, val value: Boolean) : Extra
    data class IntExtra(override val key: String, val value: Int) : Extra
    data class LongExtra(override val key: String, val value: Long) : Extra

    /** Nothing but the package: open the app's own launcher screen. */
    val opensApp: Boolean
        get() = activity == null && action == null && data == null && type == null &&
            categories.isEmpty() && extras.isEmpty()

    /** The activity's full class name. */
    val className: String?
        get() = activity?.let { if (it.startsWith(".")) pkg + it else it }

    /** content:// URIs the started app must be allowed to read. */
    val contentUris: List<String>
        get() = buildList {
            data?.takeIf { it.startsWith("content://") }?.let(::add)
            extras.filterIsInstance<StringExtra>().map { it.value }.filter { it.startsWith("content://") }.forEach(::add)
        }
}

class LaunchException(message: String) : Exception(message)

object Launch {
    const val ROM = "{rom}"

    /**
     * Parses a setup into a spec. With no template, or no game file, the app
     * is opened without a game (the user picks it in the emulator). Throws
     * [LaunchException] for a setup that can't be understood.
     */
    fun parse(exe: String, args: String = "", rom: String = ""): LaunchSpec {
        val target = exe.trim()
        if (target.isEmpty()) throw LaunchException("No app is set")
        val pkg = target.substringBefore('/')
        val activity = target.substringAfter('/', "").ifEmpty { null }
        if (!pkg.matches(Regex("[A-Za-z][A-Za-z0-9_]*(\\.[A-Za-z0-9_]+)+"))) {
            throw LaunchException("\"$pkg\" is not an Android package name")
        }
        if (args.isBlank() || rom.isBlank()) return LaunchSpec(pkg, activity)

        var spec = LaunchSpec(pkg, activity)
        val tokens = tokenize(args).map { it.replace(ROM, rom) }
        var i = 0
        fun next(flag: String): String {
            if (i >= tokens.size) throw LaunchException("$flag needs a value")
            return tokens[i++]
        }
        while (i < tokens.size) {
            when (val flag = tokens[i++]) {
                "-a" -> spec = spec.copy(action = next(flag))
                "-d" -> spec = spec.copy(data = next(flag))
                "-t" -> spec = spec.copy(type = next(flag))
                "-c" -> spec = spec.copy(categories = spec.categories + next(flag))
                "-n" -> {
                    val c = next(flag)
                    if (c.substringBefore('/') != pkg) throw LaunchException("-n names another app than $pkg")
                    spec = spec.copy(activity = c.substringAfter('/', "").ifEmpty { null })
                }
                "-e", "--es" -> spec = spec.copy(extras = spec.extras + LaunchSpec.StringExtra(next(flag), next(flag)))
                "--ez" -> {
                    val k = next(flag)
                    val v = next(flag).lowercase()
                    if (v != "true" && v != "false") throw LaunchException("--ez $k needs true or false")
                    spec = spec.copy(extras = spec.extras + LaunchSpec.BoolExtra(k, v == "true"))
                }
                "--ei" -> {
                    val k = next(flag)
                    val v = next(flag).toIntOrNull() ?: throw LaunchException("--ei $k needs a number")
                    spec = spec.copy(extras = spec.extras + LaunchSpec.IntExtra(k, v))
                }
                "--el" -> {
                    val k = next(flag)
                    val v = next(flag).toLongOrNull() ?: throw LaunchException("--el $k needs a number")
                    spec = spec.copy(extras = spec.extras + LaunchSpec.LongExtra(k, v))
                }
                else -> throw LaunchException("Unknown option $flag (use -a, -d, -t, -c, -n, -e, --ez, --ei, --el)")
            }
        }
        return spec
    }

    /** Splits a template on spaces, keeping "double" or 'single' quoted parts together. */
    fun tokenize(s: String): List<String> {
        val out = mutableListOf<String>()
        val cur = StringBuilder()
        var quote: Char? = null
        var inToken = false
        for (ch in s) {
            when {
                quote != null && ch == quote -> quote = null
                quote != null -> cur.append(ch)
                ch == '"' || ch == '\'' -> {
                    quote = ch
                    inToken = true
                }
                ch.isWhitespace() -> {
                    if (inToken) out += cur.toString()
                    cur.clear()
                    inToken = false
                }
                else -> {
                    cur.append(ch)
                    inToken = true
                }
            }
        }
        if (quote != null) throw LaunchException("Unclosed quote in the template")
        if (inToken) out += cur.toString()
        return out
    }
}

/**
 * The Intent that starts spec, or null when the app isn't installed (or has
 * no screen to open).
 */
fun LaunchSpec.toIntent(pm: PackageManager): Intent? {
    val spec = this // Intent has data/type/extras properties of its own: name the spec's explicitly
    if (spec.opensApp && spec.className == null) {
        val open = pm.getLaunchIntentForPackage(spec.pkg) ?: pm.getLeanbackLaunchIntentForPackage(spec.pkg) ?: return null
        return open.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
    val intent = Intent(spec.action ?: Intent.ACTION_MAIN)
    val cls = spec.className
    if (cls != null) intent.setClassName(spec.pkg, cls) else intent.setPackage(spec.pkg)
    val uri = spec.data?.let { Uri.parse(it) }
    when {
        uri != null && spec.type != null -> intent.setDataAndType(uri, spec.type)
        uri != null -> intent.setData(uri)
        spec.type != null -> intent.setType(spec.type)
    }
    spec.categories.forEach { intent.addCategory(it) }
    for (e in spec.extras) {
        when (e) {
            is LaunchSpec.StringExtra -> intent.putExtra(e.key, e.value)
            is LaunchSpec.BoolExtra -> intent.putExtra(e.key, e.value)
            is LaunchSpec.IntExtra -> intent.putExtra(e.key, e.value)
            is LaunchSpec.LongExtra -> intent.putExtra(e.key, e.value)
        }
    }
    val uris = spec.contentUris
    if (uris.isNotEmpty()) {
        // Lets the emulator read the game file this app was granted.
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        val clip = ClipData.newRawUri("", Uri.parse(uris.first()))
        uris.drop(1).forEach { clip.addItem(ClipData.Item(Uri.parse(it))) }
        intent.clipData = clip
    }
    if (intent.resolveActivity(pm) == null) return null
    return intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
}

/** Whether the app of an exe ("package" or "package/activity") is installed. */
fun isInstalled(pm: PackageManager, exe: String): Boolean = runCatching {
    pm.getPackageInfo(exe.substringBefore('/').trim(), 0)
}.isSuccess
