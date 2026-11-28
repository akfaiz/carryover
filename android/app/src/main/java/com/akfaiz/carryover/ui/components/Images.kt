package com.akfaiz.carryover.ui.components

import android.content.Context
import android.graphics.BitmapFactory
import android.graphics.drawable.Drawable
import android.util.LruCache
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.graphics.drawable.toBitmap
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.ui.state.initials
import com.akfaiz.carryover.ui.state.isGridImageUrl
import java.io.File
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * Decoded images by file, version and size, so scrolling the library doesn't
 * decode covers again. An eighth of the app's memory, at most.
 */
private object ImageCache {
    private val cache = object : LruCache<String, ImageBitmap>((Runtime.getRuntime().maxMemory() / 8).toInt()) {
        override fun sizeOf(key: String, value: ImageBitmap): Int = value.width * value.height * 4
    }

    fun get(key: String): ImageBitmap? = cache.get(key)
    fun put(key: String, value: ImageBitmap) {
        cache.put(key, value)
    }
}

/** Decodes an image file no larger than needed for [maxWidth] pixels; null if it can't be read. */
private fun decode(path: String, maxWidth: Int): ImageBitmap? {
    if (path.isEmpty() || !File(path).isFile) return null
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
    BitmapFactory.decodeFile(path, bounds)
    if (bounds.outWidth <= 0) return null
    var sample = 1
    while (bounds.outWidth / (sample * 2) >= maxWidth) sample *= 2
    val opts = BitmapFactory.Options().apply { inSampleSize = sample }
    return BitmapFactory.decodeFile(path, opts)?.asImageBitmap()
}

/** An image file loaded off the main thread (null while loading or when there is none). */
@Composable
fun rememberImage(path: String, version: Long, maxWidth: Int): ImageBitmap? {
    val key = "$path@$version@$maxWidth"
    val image by produceState(ImageCache.get(key), key) {
        if (value == null && path.isNotEmpty()) {
            value = withContext(Dispatchers.IO) { decode(path, maxWidth) }?.also { ImageCache.put(key, it) }
        }
    }
    return image
}

/** Reads at most [limit] bytes of a stream; null when there is more. */
internal fun readLimited(input: java.io.InputStream, limit: Int): ByteArray? {
    val out = java.io.ByteArrayOutputStream()
    val buffer = ByteArray(8192)
    while (true) {
        val n = input.read(buffer)
        if (n < 0) break
        if (out.size() + n > limit) return null
        out.write(buffer, 0, n)
    }
    return out.toByteArray()
}

/** Fetches an image from SteamGridDB (and nowhere else) no larger than needed for [maxWidth] pixels; null if it can't. */
private fun fetch(url: String, maxWidth: Int): ImageBitmap? {
    if (!isGridImageUrl(url)) return null
    val conn = java.net.URL(url).openConnection() as java.net.HttpURLConnection
    return try {
        conn.connectTimeout = 10_000
        conn.readTimeout = 15_000
        if (conn.responseCode != 200) return null
        val bytes = conn.inputStream.use { readLimited(it, 8 shl 20) } ?: return null
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
        if (bounds.outWidth <= 0) return null
        var sample = 1
        while (bounds.outWidth / (sample * 2) >= maxWidth) sample *= 2
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, BitmapFactory.Options().apply { inSampleSize = sample })?.asImageBitmap()
    } catch (e: java.io.IOException) {
        null
    } finally {
        conn.disconnect()
    }
}

/** An image of SteamGridDB loaded off the main thread (null while loading or when it can't be). */
@Composable
fun rememberRemoteImage(url: String, maxWidth: Int): ImageBitmap? {
    val key = "remote:$url@$maxWidth"
    val image by produceState(ImageCache.get(key), key) {
        if (value == null && url.isNotEmpty()) {
            value = withContext(Dispatchers.IO) { fetch(url, maxWidth) }?.also { ImageCache.put(key, it) }
        }
    }
    return image
}

/** A game's cover, or its initials on a colour of its own when it has none. */
@Composable
fun CoverImage(g: GameView, modifier: Modifier = Modifier) {
    BoxWithConstraints(modifier) {
        val px = with(LocalDensity.current) { maxWidth.roundToPx() }.coerceAtLeast(64)
        val image = rememberImage(g.coverPath, g.coverVersion, px)
        if (image != null) {
            Image(image, contentDescription = null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        } else {
            Placeholder(g.name, Modifier.fillMaxSize())
        }
    }
}

/** Wide artwork (hero, then wide, then the cover) for a game's header; null when there is none. */
@Composable
fun headerArt(g: GameView, maxWidth: Int): ImageBitmap? {
    val art = g.art["hero"] ?: g.art["wide"]
    return if (art != null) rememberImage(art.path, art.version, maxWidth) else rememberImage(g.coverPath, g.coverVersion, maxWidth)
}

/** A gradient in a hue taken from the name, with its initials (none for an empty name). Per game, so outside the theme: style:ok */
@Composable
fun Placeholder(name: String, modifier: Modifier = Modifier) {
    val hue = ((name.hashCode() % 360) + 360) % 360f
    val top = Color.hsv(hue, 0.45f, 0.55f)
    val bottom = Color.hsv((hue + 40f) % 360f, 0.55f, 0.30f)
    Box(modifier.background(Brush.verticalGradient(listOf(top, bottom))), contentAlignment = Alignment.Center) {
        if (name.isNotEmpty()) Text(
            initials(name),
            color = Color.White.copy(alpha = 0.92f), // style:ok
            fontSize = 28.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

/** An installed app's icon, loaded off the main thread. */
@Composable
fun rememberAppIcon(pkg: String): ImageBitmap? {
    val ctx = LocalContext.current
    val icon by produceState<ImageBitmap?>(ImageCache.get("app:$pkg"), pkg) {
        if (value == null && pkg.isNotEmpty()) {
            value = withContext(Dispatchers.IO) {
                runCatching { appIcon(ctx, pkg)?.toBitmap(96, 96)?.asImageBitmap() }.getOrNull()
            }?.also { ImageCache.put("app:$pkg", it) }
        }
    }
    return icon
}

private fun appIcon(ctx: Context, pkg: String): Drawable? =
    runCatching { ctx.packageManager.getApplicationIcon(pkg) }.getOrNull()

/** An app's icon, or a neutral tile when it isn't installed or chosen. */
@Composable
fun AppIcon(pkg: String, sizeDp: Int) {
    val icon = rememberAppIcon(pkg)
    if (icon != null) {
        Image(icon, null, Modifier.size(sizeDp.dp))
        return
    }
    Surface(Modifier.size(sizeDp.dp), shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.surfaceContainerHighest) {
        Box(contentAlignment = Alignment.Center) {
            Icon(Icons.Outlined.Apps, null, Modifier.size((sizeDp / 2).dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
