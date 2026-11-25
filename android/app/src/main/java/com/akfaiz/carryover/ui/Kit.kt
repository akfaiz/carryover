package com.akfaiz.carryover.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.FlowRowScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.akfaiz.carryover.R

/*
 * The app's building blocks, the Android twin of the desktop's ui/kit: one set of
 * buttons, callouts, badges, cards, empty states and page frames, so every screen
 * looks the same. Screens use these instead of Material's own (StyleTest checks).
 */

// Icons

/** The one filled icon: Play is a solid triangle on the desktop too, and the outline reads as faint on a button. */
val PlayIcon: ImageVector get() = Icons.Filled.PlayArrow // style:ok

// Tone

/** What a colour means: the desktop's badge kinds (ui/kit/badgekind.go). */
enum class Tone { Neutral, Ok, Warn, Err, Info }

@Composable
fun Tone.color(): Color {
    val s = LocalStatusColors.current
    return when (this) {
        Tone.Neutral -> MaterialTheme.colorScheme.onSurfaceVariant
        Tone.Ok -> s.ok
        Tone.Warn -> s.warn
        Tone.Err -> s.err
        Tone.Info -> s.info
    }
}

// Buttons

/** The desktop's button kinds: the page's main action, a secondary one, a destructive one, and a quiet one. */
enum class BtnKind { Primary, Secondary, Danger, Ghost }

/**
 * A button in the app's one style: 8dp corners, an optional icon 8dp before the label,
 * [large] for the page's one main action (Play, Add).
 */
@Composable
fun AppButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: BtnKind = BtnKind.Primary,
    icon: ImageVector? = null,
    enabled: Boolean = true,
    large: Boolean = false,
    content: @Composable RowScope.() -> Unit,
) {
    val shape = MaterialTheme.shapes.small
    val mod = if (large) modifier.heightIn(min = 52.dp) else modifier
    val inner: @Composable RowScope.() -> Unit = {
        if (icon != null) {
            Icon(icon, contentDescription = null, Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
        }
        content()
    }
    when (kind) {
        BtnKind.Primary -> Button(onClick, mod, enabled, shape = shape, content = inner)
        BtnKind.Secondary -> FilledTonalButton(onClick, mod, enabled, shape = shape, content = inner)
        BtnKind.Ghost -> TextButton(onClick, mod, enabled, shape = shape, content = inner)
        BtnKind.Danger -> {
            val c = Tone.Err.color()
            Button(
                onClick,
                mod,
                enabled,
                shape = shape,
                colors = ButtonDefaults.buttonColors(containerColor = c.copy(alpha = 0.15f), contentColor = c),
                content = inner,
            )
        }
    }
}

/** A button with a text label (most of them). */
@Composable
fun AppButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: BtnKind = BtnKind.Primary,
    icon: ImageVector? = null,
    enabled: Boolean = true,
    large: Boolean = false,
) {
    AppButton(onClick, modifier, kind, icon, enabled, large) { Text(text, maxLines = 1, overflow = TextOverflow.Ellipsis) }
}

/** A spinner: [small] (18dp) inside a button or a line of text, otherwise 24dp. */
@Composable
fun Spinner(modifier: Modifier = Modifier, small: Boolean = false) {
    CircularProgressIndicator(modifier.size(if (small) 18.dp else 24.dp), strokeWidth = 2.dp)
}

// Messages

/**
 * Something the user should notice: a box tinted by [tone] with an icon (the desktop's
 * kit.Callout). [content] and then [actions] sit under the text.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun Callout(
    tone: Tone,
    icon: ImageVector,
    text: String,
    modifier: Modifier = Modifier,
    content: (@Composable ColumnScope.() -> Unit)? = null,
    actions: (@Composable FlowRowScope.() -> Unit)? = null,
) {
    val c = tone.color()
    Surface(
        modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = c.copy(alpha = 0.10f),
        border = BorderStroke(1.dp, c.copy(alpha = 0.30f)),
    ) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Icon(icon, contentDescription = null, Modifier.size(18.dp), tint = c)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(text, style = MaterialTheme.typography.bodyMedium)
                content?.invoke(this)
                if (actions != null) {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), content = actions)
                }
            }
        }
    }
}

/** A small tinted pill with a word (and an icon) in the colour of [tone]: the desktop's kit.Badge. */
@Composable
fun Pill(text: String, tone: Tone, modifier: Modifier = Modifier, icon: ImageVector? = null) {
    val c = tone.color()
    Surface(modifier, shape = CircleShape, color = c.copy(alpha = 0.15f)) {
        Row(
            Modifier.padding(horizontal = 8.dp, vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            if (icon != null) Icon(icon, contentDescription = null, Modifier.size(12.dp), tint = c)
            Text(text, style = MaterialTheme.typography.labelSmall, color = c, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** A line of small grey help under a field or a title. */
@Composable
fun Hint(text: String, modifier: Modifier = Modifier) {
    Text(text, modifier, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
}

/**
 * A centred message for an empty page, with the buttons that fill it under it (the
 * desktop's kit.EmptyState).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun EmptyState(
    icon: ImageVector,
    title: String,
    text: String,
    modifier: Modifier = Modifier,
    actions: (@Composable FlowRowScope.() -> Unit)? = null,
) {
    Column(
        modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(
            Modifier.size(56.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary.copy(alpha = 0.14f)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, contentDescription = null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.primary)
        }
        Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        Text(
            text,
            Modifier.widthIn(max = 520.dp),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        if (actions != null) {
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                content = actions,
            )
        }
    }
}

// Cards and rows

/** A titled group of content on a card (the desktop's kit.Card). */
@Composable
fun SectionCard(
    title: String?,
    modifier: Modifier = Modifier,
    action: (@Composable RowScope.() -> Unit)? = null,
    contentPadding: PaddingValues = PaddingValues(16.dp),
    gap: Dp = 12.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    Surface(
        modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceContainerLow,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Column(Modifier.padding(contentPadding), verticalArrangement = Arrangement.spacedBy(gap)) {
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

/** A card inside a card, on the page's own background (the desktop's SubCard). */
@Composable
fun SubCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Surface(
        modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.background,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(4.dp), content = content)
    }
}

/**
 * [items] one under another with a line between them (the desktop's kit.ListRow),
 * inside a [SectionCard].
 */
@Composable
fun <T> ListRows(items: List<T>, modifier: Modifier = Modifier, row: @Composable ColumnScope.(T) -> Unit) {
    Column(modifier.fillMaxWidth()) {
        items.forEachIndexed { i, item ->
            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Column(Modifier.fillMaxWidth().padding(vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) { row(item) }
        }
    }
}

/** A selectable row with a radio button. */
@Composable
fun ChoiceRow(selected: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable RowScope.() -> Unit) {
    val shape = MaterialTheme.shapes.medium
    Row(
        modifier
            .fillMaxWidth()
            .focusRing(shape)
            .clip(shape)
            .selectable(selected = selected, enabled = enabled, onClick = onClick)
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        RadioButton(selected = selected, onClick = null, enabled = enabled)
        content()
    }
}

// Pages

/** The bar of every page that is not a tab: the title and, when [onBack] is given, a back arrow. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PageTopBar(title: String, onBack: (() -> Unit)? = null, actions: @Composable RowScope.() -> Unit = {}) {
    TopAppBar(
        title = { Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        navigationIcon = {
            if (onBack != null) {
                IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.action_back))
                }
            }
        },
        actions = actions,
        windowInsets = pageBarInsets(),
    )
}

/**
 * The scrolling body of a page: centred, at most 760dp wide, with the same gutters and
 * gaps on every page. [padding] is the Scaffold's.
 */
@Composable
fun PageColumn(padding: PaddingValues, modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Column(
        Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Column(
            modifier.widthIn(max = 760.dp).fillMaxWidth().padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
            content = content,
        )
    }
}
