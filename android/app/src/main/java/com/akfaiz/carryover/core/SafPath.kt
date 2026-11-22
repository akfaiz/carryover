package com.akfaiz.carryover.core

/**
 * The path form of a save folder granted through the Storage Access
 * Framework: ":saf:<authority>/<tree document id, percent-encoded>[/<sub/path>]".
 * The Go core stores and syncs it like any path; [SafStorage] resolves it.
 * Pure Kotlin, so it is unit tested without Android.
 */
object SafPath {
    const val PREFIX = ":saf:"
    private const val EXTERNAL_STORAGE = "com.android.externalstorage.documents"

    data class Parts(val authority: String, val treeDocId: String, val rel: List<String>)

    fun isSaf(path: String): Boolean = path.startsWith(PREFIX)

    /** The path of a granted tree. */
    fun of(authority: String, treeDocId: String): String = PREFIX + authority + "/" + encode(treeDocId)

    /** Splits a path; null when it isn't one of ours. */
    fun parse(path: String): Parts? {
        if (!isSaf(path)) return null
        val segs = path.removePrefix(PREFIX).split('/').filter { it.isNotEmpty() }
        if (segs.size < 2) return null
        return Parts(segs[0], decode(segs[1]), segs.drop(2))
    }

    /**
     * A readable name: "Internal storage › Emu/saves" for device storage,
     * "<provider> › <folder>" otherwise (providerLabel names the app that
     * publishes the folder, such as an emulator).
     */
    fun describe(path: String, providerLabel: (String) -> String? = { null }): String {
        val p = parse(path) ?: return path
        val (where, folder) = if (p.authority == EXTERNAL_STORAGE) {
            val volume = p.treeDocId.substringBefore(':')
            (if (volume == "primary") "Internal storage" else "SD card $volume") to p.treeDocId.substringAfter(':', "")
        } else {
            (providerLabel(p.authority) ?: p.authority) to p.treeDocId.substringAfterLast(':')
        }
        val full = (listOf(folder) + p.rel).filter { it.isNotEmpty() }.joinToString("/")
        return if (full.isEmpty()) where else "$where › $full"
    }

    private const val SAFE = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

    /** Percent-encodes everything but unreserved characters, so the id is one path segment. */
    fun encode(s: String): String = buildString {
        for (b in s.toByteArray(Charsets.UTF_8)) {
            val c = (b.toInt() and 0xff).toChar()
            if (c in SAFE) append(c) else append('%').append("%02X".format(b.toInt() and 0xff))
        }
    }

    fun decode(s: String): String {
        val out = java.io.ByteArrayOutputStream()
        var i = 0
        while (i < s.length) {
            val c = s[i]
            if (c == '%' && i + 2 < s.length) {
                out.write(s.substring(i + 1, i + 3).toInt(16))
                i += 3
            } else {
                out.write(c.toString().toByteArray(Charsets.UTF_8))
                i++
            }
        }
        return out.toString(Charsets.UTF_8.name())
    }
}
