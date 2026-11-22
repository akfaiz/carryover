package com.akfaiz.carryover.ui

import android.content.Context
import android.graphics.BitmapFactory
import android.graphics.drawable.Drawable
import android.icu.text.RelativeDateTimeFormatter
import android.icu.util.ULocale
import android.text.format.DateUtils
import android.util.LruCache
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.CloudDone
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.graphics.drawable.toBitmap
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.Summary
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

// Images

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

/** A gradient in a hue taken from the name, with its initials (none for an empty name). */
@Composable
fun Placeholder(name: String, modifier: Modifier = Modifier) {
    val hue = ((name.hashCode() % 360) + 360) % 360f
    val top = Color.hsv(hue, 0.45f, 0.55f)
    val bottom = Color.hsv((hue + 40f) % 360f, 0.55f, 0.30f)
    Box(modifier.background(Brush.verticalGradient(listOf(top, bottom))), contentAlignment = Alignment.Center) {
        if (name.isNotEmpty()) Text(
            initials(name),
            color = Color.White.copy(alpha = 0.92f),
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

// Status

@Composable
fun Badge.color(): Color {
    val s = LocalStatusColors.current
    return when (this) {
        Badge.Playing, Badge.Busy -> s.info
        Badge.Conflict, Badge.Changed -> s.warn
        Badge.Error -> s.err
        Badge.Synced -> s.ok
        Badge.Never, Badge.NotSetUp -> MaterialTheme.colorScheme.onSurfaceVariant
    }
}

val Badge.icon: ImageVector
    get() = when (this) {
        Badge.Playing -> Icons.Outlined.PlayArrow
        Badge.Busy -> Icons.Outlined.Sync
        Badge.Conflict -> Icons.Outlined.WarningAmber
        Badge.Error -> Icons.Outlined.ErrorOutline
        Badge.Changed -> Icons.Outlined.CloudUpload
        Badge.Synced -> Icons.Outlined.CloudDone
        Badge.Never -> Icons.Outlined.CloudOff
        Badge.NotSetUp -> Icons.Outlined.Build
    }

@Composable
fun Badge.label(): String = stringResource(
    when (this) {
        Badge.Playing -> R.string.status_playing
        Badge.Busy -> R.string.status_syncing
        Badge.Conflict -> R.string.status_conflict
        Badge.Error -> R.string.status_error
        Badge.Changed -> R.string.status_changed
        Badge.Synced -> R.string.status_synced
        Badge.Never -> R.string.status_never
        Badge.NotSetUp -> R.string.status_not_set_up
    },
)

/** A small pill with the status' icon and name, readable over a cover. */
@Composable
fun StatusPill(badge: Badge, modifier: Modifier = Modifier, iconOnly: Boolean = false) {
    val color = badge.color()
    Surface(modifier, shape = CircleShape, color = MaterialTheme.colorScheme.surface.copy(alpha = 0.92f)) {
        if (iconOnly) {
            // On a cover: the words are in the line under it, and would not fit in every language.
            Icon(badge.icon, contentDescription = badge.label(), Modifier.padding(5.dp).size(16.dp), tint = color)
            return@Surface
        }
        Row(
            Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Icon(badge.icon, contentDescription = null, Modifier.size(14.dp), tint = color)
            Text(
                badge.label(),
                style = MaterialTheme.typography.labelSmall,
                color = color,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** The status of a profile in a sentence. */
@Composable
fun statusLine(s: GameStatus): String = when {
    s.conflict -> stringResource(R.string.status_line_conflict)
    s.lastError.isNotEmpty() -> stringResource(R.string.status_line_error, s.lastError)
    s.localChanges -> stringResource(R.string.status_line_changed)
    s.lastSyncAt.isNotEmpty() -> stringResource(R.string.status_line_synced, relativeTime(s.lastSyncAt))
    else -> stringResource(R.string.status_line_never)
}

/**
 * An RFC 3339 time from the core as "5 minutes ago" (a date after a week), in
 * the app's language: the activity's locale, which follows the app's language
 * setting where the process default (used by DateUtils) may not. As given if
 * it doesn't parse.
 */
@Composable
fun relativeTime(rfc3339: String): String {
    val millis = runCatching { java.time.OffsetDateTime.parse(rfc3339).toInstant().toEpochMilli() }.getOrNull()
        ?: return rfc3339
    val diff = System.currentTimeMillis() - millis
    if (diff < DateUtils.MINUTE_IN_MILLIS) return stringResource(R.string.time_just_now)
    val locale = LocalConfiguration.current.locales[0]
    val f = RelativeDateTimeFormatter.getInstance(ULocale.forLocale(locale))
    fun ago(n: Long, unit: RelativeDateTimeFormatter.RelativeUnit) =
        f.format(n.toDouble(), RelativeDateTimeFormatter.Direction.LAST, unit)
    return when {
        diff < DateUtils.HOUR_IN_MILLIS -> ago(diff / DateUtils.MINUTE_IN_MILLIS, RelativeDateTimeFormatter.RelativeUnit.MINUTES)
        diff < DateUtils.DAY_IN_MILLIS -> ago(diff / DateUtils.HOUR_IN_MILLIS, RelativeDateTimeFormatter.RelativeUnit.HOURS)
        diff < DateUtils.WEEK_IN_MILLIS -> ago(diff / DateUtils.DAY_IN_MILLIS, RelativeDateTimeFormatter.RelativeUnit.DAYS)
        else -> java.text.DateFormat.getDateInstance(java.text.DateFormat.MEDIUM, locale).format(java.util.Date(millis))
    }
}

/** "12 files, 3.4 MB, from Desk PC". */
@Composable
fun describe(s: Summary): String {
    val files = pluralStringResource(R.plurals.files_size, s.files, s.files, formatBytes(s.bytes))
    return if (s.deviceName.isNotEmpty()) stringResource(R.string.conflict_from, files, s.deviceName) else files
}

// Layout

/**
 * A visible ring while focused (D-pad, gamepad, keyboard): Material's own
 * focus highlight is too faint to follow from a sofa.
 */
fun Modifier.focusRing(shape: Shape = RoundedCornerShape(14.dp)): Modifier = composed {
    var focused by remember { mutableStateOf(false) }
    val ring = MaterialTheme.colorScheme.primary
    this
        .onFocusChanged { focused = it.hasFocus }
        .then(if (focused) Modifier.border(3.dp, ring, shape) else Modifier)
}

/**
 * Lets the D-pad leave a single-line text field: a text field keeps up and
 * down for itself, which traps a gamepad in it.
 */
fun Modifier.dpadExit(): Modifier = composed {
    val focus = LocalFocusManager.current
    onPreviewKeyEvent { e ->
        if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
        when (e.key) {
            Key.DirectionDown -> focus.moveFocus(FocusDirection.Down)
            Key.DirectionUp -> focus.moveFocus(FocusDirection.Up)
            else -> false
        }
    }
}

/** A titled group of content on a card. */
@Composable
fun SectionCard(
    title: String?,
    modifier: Modifier = Modifier,
    action: (@Composable RowScope.() -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    Card(
        modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow),
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            if (title != null || action != null) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        title.orEmpty(),
                        style = MaterialTheme.typography.titleMedium,
                        modifier = Modifier.weight(1f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    action?.invoke(this)
                }
            }
            content()
        }
    }
}

/** A heading above a group of setting rows. */
@Composable
fun SectionHeader(text: String, modifier: Modifier = Modifier) {
    Text(
        text,
        style = MaterialTheme.typography.labelLarge,
        color = MaterialTheme.colorScheme.primary,
        modifier = modifier.padding(start = 16.dp, top = 20.dp, bottom = 6.dp),
    )
}

/** A row of a settings list: icon, title, a line of detail and something on the right. */
@Composable
fun SettingRow(
    icon: ImageVector?,
    title: String,
    detail: String? = null,
    onClick: (() -> Unit)? = null,
    leading: (@Composable () -> Unit)? = null,
    detailLines: Int = 3,
    trailing: (@Composable () -> Unit)? = if (onClick != null) {
        { Icon(Icons.AutoMirrored.Outlined.KeyboardArrowRight, null, tint = MaterialTheme.colorScheme.onSurfaceVariant) }
    } else {
        null
    },
) {
    val shape = RoundedCornerShape(12.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.focusRing(shape).clip(shape).clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        when {
            leading != null -> leading()
            icon != null -> Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyLarge)
            detail?.takeIf { it.isNotEmpty() }?.let {
                Text(
                    it,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = detailLines,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        trailing?.invoke()
    }
}

/** A centred message for an empty page, with an optional action under it. */
@Composable
fun EmptyState(icon: ImageVector, title: String, text: String, modifier: Modifier = Modifier, action: (@Composable () -> Unit)? = null) {
    Column(
        modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 48.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(
            Modifier.size(72.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, contentDescription = null, Modifier.size(36.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
        }
        Text(title, style = MaterialTheme.typography.titleLarge)
        Text(
            text,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
        )
        action?.invoke()
    }
}
