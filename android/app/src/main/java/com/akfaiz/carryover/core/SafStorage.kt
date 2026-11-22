package com.akfaiz.carryover.core

import android.content.ContentResolver
import android.content.Context
import com.akfaiz.carryover.R
import android.database.Cursor
import android.net.Uri
import android.os.SystemClock
import android.provider.DocumentsContract
import android.provider.DocumentsContract.Document
import com.akfaiz.carryover.bridge.mobile.Storage
import kotlinx.serialization.Serializable
import java.io.FileNotFoundException
import java.io.IOException
import java.util.concurrent.ConcurrentHashMap

/**
 * Save folders granted through the Storage Access Framework, for the Go core
 * (mobile.Storage). Paths have the [SafPath] form; a sub-path is found by
 * display name, one folder level at a time, and the document ids found are
 * cached (a lookup that fails with a cached id starts again without cache).
 * Files are handed to Go as detached file descriptors, which Go closes.
 */
class SafStorage(context: Context) : Storage {
    private val app = context.applicationContext
    private val resolver: ContentResolver = context.contentResolver

    /** Document id by path, for paths resolved before. */
    private val ids = ConcurrentHashMap<String, String>()

    /**
     * Held while folders and files are created. Transfers run several at a
     * time: two creating the same new folder would both find none, and the
     * provider would name the second "folder (1)".
     */
    private val createLock = Any()

    /** The granted trees as of [grantsAt]: asking the system for them costs a call per file otherwise. */
    @Volatile
    private var grants: Set<Uri> = emptySet()

    @Volatile
    private var grantsAt = 0L

    private fun isGranted(tree: Uri): Boolean {
        val now = SystemClock.elapsedRealtime()
        if (tree in grants && now - grantsAt < GRANTS_TTL_MS) return true
        grants = resolver.persistedUriPermissions.map { it.uri }.toSet()
        grantsAt = now
        return tree in grants
    }

    @Serializable
    private data class Entry(val name: String, val dir: Boolean, val size: Long, val modTime: Long)

    private class Target(val tree: Uri, val parts: SafPath.Parts)

    private fun target(path: String): Target {
        val parts = SafPath.parse(path) ?: throw IllegalArgumentException("not a granted folder: $path")
        val tree = DocumentsContract.buildTreeDocumentUri(parts.authority, parts.treeDocId)
        if (!isGranted(tree)) {
            throw SecurityException(app.getString(R.string.saf_lost, SafPath.describe(path)))
        }
        return Target(tree, parts)
    }

    private fun docUri(t: Target, docId: String): Uri = DocumentsContract.buildDocumentUriUsingTree(t.tree, docId)

    private val columns = arrayOf(
        Document.COLUMN_DOCUMENT_ID,
        Document.COLUMN_DISPLAY_NAME,
        Document.COLUMN_MIME_TYPE,
        Document.COLUMN_SIZE,
        Document.COLUMN_LAST_MODIFIED,
    )

    private fun entry(c: Cursor) = Entry(
        name = c.getString(1) ?: "",
        dir = c.getString(2) == Document.MIME_TYPE_DIR,
        size = if (c.isNull(3)) 0 else c.getLong(3),
        modTime = if (c.isNull(4)) 0 else c.getLong(4),
    )

    /** The child named name of folder docId, as (document id, entry). */
    private fun child(t: Target, docId: String, name: String): Pair<String, Entry>? {
        val uri = DocumentsContract.buildChildDocumentsUriUsingTree(t.tree, docId)
        resolver.query(uri, columns, null, null, null)?.use { c ->
            while (c.moveToNext()) {
                if (c.getString(1) == name) return c.getString(0) to entry(c)
            }
        }
        return null
    }

    private fun key(t: Target, rel: List<String>) = t.tree.toString() + "/" + rel.joinToString("/")

    /** The document id of a path, or null when it doesn't exist. */
    private fun resolve(t: Target, rel: List<String> = t.parts.rel, cached: Boolean = true): String? {
        if (rel.isEmpty()) return t.parts.treeDocId
        if (cached) ids[key(t, rel)]?.let { return it }
        val parent = resolve(t, rel.dropLast(1), cached) ?: return null
        val id = child(t, parent, rel.last())?.first ?: return null
        ids[key(t, rel)] = id
        return id
    }

    /** Runs block with the path's document id; once more without cache if a cached id went stale. */
    private fun <T> withDoc(t: Target, block: (String?) -> T): T = try {
        block(resolve(t))
    } catch (e: FileNotFoundException) {
        forget(t)
        block(resolve(t, cached = false))
    } catch (e: IllegalArgumentException) {
        forget(t)
        block(resolve(t, cached = false))
    }

    private fun forget(t: Target) {
        val prefix = t.tree.toString() + "/"
        ids.keys.removeIf { it.startsWith(prefix) }
    }

    private fun query(t: Target, docId: String): Entry? =
        resolver.query(docUri(t, docId), columns, null, null, null)?.use { c ->
            if (c.moveToFirst()) entry(c) else null
        }

    override fun stat(path: String): String {
        val t = target(path)
        return withDoc(t) { id -> id?.let { query(t, it) } }?.let { json.encodeToString(it) } ?: ""
    }

    override fun list(path: String): String {
        val t = target(path)
        val entries: List<Entry> = withDoc<List<Entry>?>(t) { id ->
            if (id == null) return@withDoc null
            val uri = DocumentsContract.buildChildDocumentsUriUsingTree(t.tree, id)
            resolver.query(uri, columns, null, null, null)?.use { c ->
                buildList<Entry> { while (c.moveToNext()) add(entry(c)) }
            }
        } ?: return ""
        return json.encodeToString(entries)
    }

    override fun openRead(path: String): Long {
        val t = target(path)
        return withDoc(t) { id ->
            id ?: throw FileNotFoundException(path)
            val pfd = resolver.openFileDescriptor(docUri(t, id), "r") ?: throw FileNotFoundException(path)
            pfd.detachFd().toLong()
        }
    }

    override fun openWrite(path: String): Long {
        val t = target(path)
        val rel = t.parts.rel
        if (rel.isEmpty()) throw IllegalArgumentException("not a file: $path")
        val id = synchronized(createLock) {
            val parent = mkdirs(t, rel.dropLast(1))
            val name = rel.last()
            child(t, parent, name)?.first ?: create(t, parent, name, "application/octet-stream")
        }
        ids[key(t, rel)] = id
        // "wt" truncates; "w" alone may leave the end of a longer old file.
        val pfd = resolver.openFileDescriptor(docUri(t, id), "wt") ?: throw FileNotFoundException(path)
        return pfd.detachFd().toLong()
    }

    override fun remove(path: String) {
        val t = target(path)
        withDoc(t) { id ->
            id ?: throw FileNotFoundException(path)
            // deleteDocument removes a whole folder tree: the contract here is a file or an empty folder.
            if (query(t, id)?.dir == true && hasChildren(t, id)) throw IOException("folder is not empty: $path")
            if (!DocumentsContract.deleteDocument(resolver, docUri(t, id))) throw FileNotFoundException(path)
        }
        ids.keys.removeIf { it == key(t, t.parts.rel) || it.startsWith(key(t, t.parts.rel) + "/") }
    }

    override fun mkdirAll(path: String) {
        val t = target(path)
        synchronized(createLock) { mkdirs(t, t.parts.rel) }
    }

    private fun hasChildren(t: Target, docId: String): Boolean {
        val uri = DocumentsContract.buildChildDocumentsUriUsingTree(t.tree, docId)
        return resolver.query(uri, arrayOf(Document.COLUMN_DOCUMENT_ID), null, null, null)?.use { it.moveToFirst() } ?: false
    }

    /** Creates the folders of rel as needed and returns the last one's id. Call with [createLock] held. */
    private fun mkdirs(t: Target, rel: List<String>): String {
        var id = t.parts.treeDocId
        for (i in rel.indices) {
            val sub = rel.subList(0, i + 1)
            val known = ids[key(t, sub)] ?: child(t, id, rel[i])?.let { (childId, e) ->
                if (!e.dir) throw IllegalStateException("${rel[i]} is a file, not a folder")
                childId
            }
            id = known ?: create(t, id, rel[i], Document.MIME_TYPE_DIR)
            ids[key(t, sub)] = id
        }
        return id
    }

    /** Creates a document; fails if the provider gave it another name (some add an extension). */
    private fun create(t: Target, parentId: String, name: String, mime: String): String {
        val uri = DocumentsContract.createDocument(resolver, docUri(t, parentId), mime, name)
            ?: throw FileNotFoundException("couldn't create $name")
        val id = DocumentsContract.getDocumentId(uri)
        val got = query(t, id)?.name
        if (got != null && got != name) {
            val renamed = runCatching { DocumentsContract.renameDocument(resolver, uri, name) }.getOrNull()
                ?: run {
                    DocumentsContract.deleteDocument(resolver, uri)
                    throw IllegalStateException("the folder's app saved $name as $got")
                }
            return DocumentsContract.getDocumentId(renamed)
        }
        return id
    }

    private companion object {
        /** How long the list of granted trees is trusted. */
        const val GRANTS_TTL_MS = 5_000L
    }
}
