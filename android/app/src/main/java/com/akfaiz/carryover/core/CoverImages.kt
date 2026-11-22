package com.akfaiz.carryover.core

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

/** Makes the image a user picked fit a game's cover. */
object CoverImages {
    /** The tallest a cover is kept (a Steam cover's size), in pixels. */
    const val MAX_HEIGHT = 900

    /**
     * Reads the image through its content:// URI, scales it to at most
     * [MAX_HEIGHT] tall and writes it as a JPEG in the cache, which the core
     * shares with the other devices. The caller deletes the file.
     */
    suspend fun prepare(ctx: Context, uri: Uri): File = withContext(Dispatchers.IO) {
        val bitmap = ctx.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it) }
            ?: error("unreadable image")
        val scaled = if (bitmap.height > MAX_HEIGHT) {
            Bitmap.createScaledBitmap(bitmap, bitmap.width * MAX_HEIGHT / bitmap.height, MAX_HEIGHT, true)
        } else {
            bitmap
        }
        File(ctx.cacheDir, "cover.jpg").also { f ->
            f.outputStream().use { scaled.compress(Bitmap.CompressFormat.JPEG, 90, it) }
        }
    }
}
