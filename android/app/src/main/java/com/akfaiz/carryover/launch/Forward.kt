package com.akfaiz.carryover.launch

import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.SafPath

/**
 * The parts of a launch from a frontend that need no Android: which file a
 * launch is for, which game profile that is, and whether Carryover may
 * forward it. Unit tested without Android.
 */
object Forward {
    /** Extras Carryover reads (and doesn't pass on). */
    const val EXTRA_TARGET = "carryover.target"
    const val EXTRA_KEY = "carryover.key"

    /** With carryover.key: the game is already running, only sync and watch it (no launch). */
    const val EXTRA_TRACK = "carryover.track"
    const val EXTRA_PREFIX = "carryover."

    private const val EXTERNAL_STORAGE = "com.android.externalstorage.documents"

    /**
     * A file reference made comparable: a path stays a path, file:// becomes
     * its path, a document URI of device storage the path it stands for, and
     * any other document URI "content:<authority>:<document id>". Null for
     * text that isn't a file reference.
     */
    fun normalize(ref: String): String? {
        val s = ref.trim()
        return when {
            s.startsWith("/") -> s.trimEnd('/')
            s.startsWith("file://") -> SafPath.decode(s.removePrefix("file://")).trimEnd('/')
            s.startsWith("content://") -> {
                val rest = s.removePrefix("content://")
                val authority = rest.substringBefore('/')
                val segs = rest.substringAfter('/', "").split('/')
                // .../document/<id> or .../tree/<tree>/document/<id>
                val i = segs.lastIndexOf("document")
                if (i < 0 || i + 1 >= segs.size) return "content:$authority:${rest.substringAfter('/', "")}"
                val docId = SafPath.decode(segs[i + 1])
                if (authority == EXTERNAL_STORAGE && ':' in docId) {
                    val volume = docId.substringBefore(':')
                    val path = docId.substringAfter(':')
                    val root = if (volume == "primary") "/storage/emulated/0" else "/storage/$volume"
                    if (path.isEmpty()) root else "$root/$path"
                } else {
                    "content:$authority:$docId"
                }
            }
            else -> null
        }
    }

    /** The file references in a launch: its data and the string extras that look like files. */
    fun roms(data: String?, extras: Map<String, String>): List<String> =
        (listOfNotNull(data) + extras.filterKeys { !it.startsWith(EXTRA_PREFIX) }.values)
            .filter { normalize(it) != null }

    /** Equal paths are the same file; /sdcard is the primary storage. */
    private fun canonical(p: String): String = when {
        p == "/sdcard" || p.startsWith("/sdcard/") -> "/storage/emulated/0" + p.removePrefix("/sdcard")
        p.startsWith("/storage/self/primary") -> "/storage/emulated/0" + p.removePrefix("/storage/self/primary")
        else -> p
    }

    fun sameFile(a: String, b: String): Boolean {
        val na = normalize(a)?.let(::canonical) ?: return false
        val nb = normalize(b)?.let(::canonical) ?: return false
        return na == nb
    }

    /** A game profile a launch can be for. */
    data class Candidate(val game: GameView, val profile: Profile) {
        val title: String get() = if (profile.emulator == null) game.name else "${game.name} (${profile.emulator})"
    }

    /** The app package of an exe ("package" or "package/activity"). */
    fun packageOf(exe: String): String = exe.substringBefore('/').trim()

    /**
     * The profiles played with the app pkg on this device: emulator profiles
     * whose emulator is that app, and games whose own app it is.
     */
    fun candidates(games: List<GameView>, emulators: List<EmulatorView>, pkg: String): List<Candidate> {
        val emuIds = emulators.filter { packageOf(it.device.exe) == pkg && pkg.isNotEmpty() }.map { it.id }.toSet()
        return games.flatMap { g ->
            g.profiles.filter { p ->
                if (p.emulatorId == null) packageOf(p.app) == pkg && pkg.isNotEmpty() else p.emulatorId in emuIds
            }.map { Candidate(g, it) }
        }
    }

    /**
     * The profile a launch is for: the one whose game file is a file of the
     * launch; else, for a game's own app, the only game using it. Null when
     * it can't be told (the user is asked).
     */
    fun match(candidates: List<Candidate>, roms: List<String>): Candidate? {
        candidates.firstOrNull { c -> c.profile.rom.isNotEmpty() && roms.any { sameFile(it, c.profile.rom) } }
            ?.let { return it }
        val native = candidates.filter { it.profile.emulatorId == null }
        return native.singleOrNull()?.takeIf { candidates.size == 1 }
    }

    /**
     * The apps Carryover may forward a launch to: those set up on this device
     * as an emulator or as a game's own app, never Carryover itself. An
     * exported Carryover must not start arbitrary activities for any caller.
     */
    fun allowedTargets(games: List<GameView>, emulators: List<EmulatorView>, ownPackage: String): Set<String> =
        (emulators.map { packageOf(it.device.exe) } + games.flatMap { g -> g.profiles.map { packageOf(it.app) } })
            .filter { it.isNotEmpty() && it != ownPackage }
            .toSet()

    /**
     * Whether a URI may be granted on to the target: only when the caller
     * granted it to Carryover with the launch, and never one inside a folder
     * Carryover keeps a permanent grant to (its save folders).
     */
    fun mayGrant(uri: String, grantedByCaller: Boolean, persistedTrees: List<String>): Boolean {
        if (!grantedByCaller || !uri.startsWith("content://")) return false
        return persistedTrees.none { tree -> uri == tree || uri.startsWith("$tree/") || isInside(uri, tree) }
    }

    /** Whether document uri lies in the tree (both as normalized paths or document ids). */
    private fun isInside(uri: String, tree: String): Boolean {
        val u = normalize(uri)?.let(::canonical) ?: return false
        val t = treeRoot(tree)?.let(::canonical) ?: return false
        return u == t || u.startsWith("$t/") || u.startsWith("$t:")
    }

    /** A tree URI's root as normalize would give it for a document. */
    private fun treeRoot(tree: String): String? {
        if (!tree.startsWith("content://")) return null
        val rest = tree.removePrefix("content://")
        val authority = rest.substringBefore('/')
        val segs = rest.substringAfter('/', "").split('/')
        val i = segs.indexOf("tree")
        if (i < 0 || i + 1 >= segs.size) return null
        return normalize("content://$authority/document/${segs[i + 1]}")
    }
}
