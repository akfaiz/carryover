package com.akfaiz.carryover.ui.components

import android.icu.text.RelativeDateTimeFormatter
import android.icu.util.ULocale
import android.text.format.DateUtils
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.CloudDone
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.GameStatus
import com.akfaiz.carryover.core.Summary
import com.akfaiz.carryover.ui.kit.Pill
import com.akfaiz.carryover.ui.kit.Tone
import com.akfaiz.carryover.ui.kit.color
import com.akfaiz.carryover.ui.state.Badge
import com.akfaiz.carryover.ui.state.formatBytes

/** The colour of a status, as on the desktop (components/statusof.go): a conflict is an error, a game that needs setup a warning. */
val Badge.tone: Tone
    get() = when (this) {
        Badge.Playing, Badge.Busy -> Tone.Info
        Badge.Conflict, Badge.Error -> Tone.Err
        Badge.Changed, Badge.NotSetUp -> Tone.Warn
        Badge.Synced -> Tone.Ok
        Badge.Never -> Tone.Neutral
    }

@Composable
fun Badge.color(): Color = tone.color()

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

/** The status as the desktop's badge: a tinted pill with the icon and the word. */
@Composable
fun StatusBadge(badge: Badge, modifier: Modifier = Modifier) {
    Pill(badge.label(), badge.tone, modifier, badge.icon)
}

/** The status as an icon alone on an opaque pill, readable over a cover (the words are in the line under it). */
@Composable
fun StatusDot(badge: Badge, modifier: Modifier = Modifier) {
    Surface(modifier, shape = CircleShape, color = MaterialTheme.colorScheme.surface.copy(alpha = 0.92f)) {
        Icon(badge.icon, contentDescription = badge.label(), Modifier.padding(5.dp).size(16.dp), tint = badge.color())
    }
}

/** The status of a profile in a sentence. */
@Composable
fun statusLine(s: GameStatus): String = when {
    s.conflict -> stringResource(R.string.status_line_conflict)
    s.lastError.isNotEmpty() -> stringResource(R.string.status_line_error, coreText(s.lastError, s.lastErrorMsg))
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
