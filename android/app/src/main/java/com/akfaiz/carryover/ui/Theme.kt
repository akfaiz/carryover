package com.akfaiz.carryover.ui

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Status colours (as on the desktop: ui/kit/palette.go), outside Material's roles. */
@Immutable
data class StatusColors(val ok: Color, val warn: Color, val err: Color, val info: Color)

val LocalStatusColors = staticCompositionLocalOf {
    StatusColors(ok = Color(0xFF128A5A), warn = Color(0xFFB36B00), err = Color(0xFFD03030), info = Color(0xFF2F62E0))
}

// The desktop palette (ui/kit/palette.go: Bg, Bg2 = cards, Bg3 = raised, Bg4 = pressed) as Material 3 roles.
private val Light = lightColorScheme(
    primary = Color(0xFF2F62E0),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFDCE5FD),
    onPrimaryContainer = Color(0xFF0E2A73),
    secondary = Color(0xFF5F6578),
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFE0E3EC),
    onSecondaryContainer = Color(0xFF1B1E28),
    tertiary = Color(0xFF128A5A),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFCFEFDF),
    onTertiaryContainer = Color(0xFF053D27),
    error = Color(0xFFD03030),
    onError = Color.White,
    errorContainer = Color(0xFFFADADA),
    onErrorContainer = Color(0xFF5C0E0E),
    background = Color(0xFFF5F6F9),
    onBackground = Color(0xFF1B1E28),
    surface = Color(0xFFF5F6F9),
    onSurface = Color(0xFF1B1E28),
    surfaceVariant = Color(0xFFE0E3EC),
    onSurfaceVariant = Color(0xFF5F6578),
    surfaceContainerLowest = Color(0xFFFFFFFF),
    surfaceContainerLow = Color(0xFFE9EBF1),
    surfaceContainer = Color(0xFFE9EBF1),
    surfaceContainerHigh = Color(0xFFE0E3EC),
    surfaceContainerHighest = Color(0xFFD2D6E2),
    outline = Color(0xFFC9CDD9),
    outlineVariant = Color(0xFFDCDFE8),
    inverseSurface = Color(0xFF1D2029),
    inverseOnSurface = Color(0xFFE7E9EE),
    inversePrimary = Color(0xFF5B8CFF),
)

private val Dark = darkColorScheme(
    primary = Color(0xFF5B8CFF),
    // Not the desktop's white: on the lighter blue of the dark theme it is too faint to read at 4.5:1.
    onPrimary = Color(0xFF041A4D),
    primaryContainer = Color(0xFF1E3B85),
    onPrimaryContainer = Color(0xFFDCE5FD),
    secondary = Color(0xFF8B90A0),
    onSecondary = Color(0xFF0E0F14),
    secondaryContainer = Color(0xFF20232D),
    onSecondaryContainer = Color(0xFFE7E9EE),
    tertiary = Color(0xFF3CCF91),
    onTertiary = Color(0xFF00321D),
    tertiaryContainer = Color(0xFF0E4A32),
    onTertiaryContainer = Color(0xFFCFEFDF),
    error = Color(0xFFFF6B6B),
    onError = Color(0xFF450808),
    errorContainer = Color(0xFF5C1717),
    onErrorContainer = Color(0xFFFADADA),
    background = Color(0xFF0E0F14),
    onBackground = Color(0xFFE7E9EE),
    surface = Color(0xFF0E0F14),
    onSurface = Color(0xFFE7E9EE),
    surfaceVariant = Color(0xFF20232D),
    onSurfaceVariant = Color(0xFF8B90A0),
    surfaceContainerLowest = Color(0xFF0A0B0F),
    surfaceContainerLow = Color(0xFF15171F),
    surfaceContainer = Color(0xFF15171F),
    surfaceContainerHigh = Color(0xFF20232D),
    surfaceContainerHighest = Color(0xFF2A2E3A),
    outline = Color(0xFF2A2E3A),
    outlineVariant = Color(0xFF2A2E3A),
    inverseSurface = Color(0xFFE7E9EE),
    inverseOnSurface = Color(0xFF1B1E28),
    inversePrimary = Color(0xFF2F62E0),
)

private val LightStatus = StatusColors(Color(0xFF128A5A), Color(0xFFB36B00), Color(0xFFD03030), Color(0xFF2F62E0))
private val DarkStatus = StatusColors(Color(0xFF3CCF91), Color(0xFFF5B84B), Color(0xFFFF6B6B), Color(0xFF5B8CFF))

// The desktop's radii (ui/kit/tokens.go): RadiusSM 6, Radius 8, RadiusLG 12; extraLarge is its pill/chip radius.
private val AppShapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp),
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(12.dp),
    extraLarge = RoundedCornerShape(18.dp),
)

// The desktop's weights (ui/kit/widgets.go): card and dialog titles 18 semibold, badges 11 semibold, buttons semibold.
private val AppTypography = Typography().let {
    it.copy(
        titleMedium = it.titleMedium.copy(fontSize = 18.sp, lineHeight = 24.sp, fontWeight = FontWeight.SemiBold),
        titleSmall = it.titleSmall.copy(fontWeight = FontWeight.SemiBold),
        labelLarge = it.labelLarge.copy(fontWeight = FontWeight.SemiBold),
        labelSmall = it.labelSmall.copy(fontSize = 11.sp, lineHeight = 16.sp, fontWeight = FontWeight.SemiBold),
    )
}

/** Whether the theme setting ("", "light", "dark") means dark here. */
@Composable
fun isDark(theme: String): Boolean = when (theme) {
    "dark" -> true
    "light" -> false
    else -> isSystemInDarkTheme()
}

/**
 * Carryover's look: the desktop palette, or the wallpaper's colours (Android 12+)
 * when the user chose them. [theme] is the setting: "", "light" or "dark".
 */
@Composable
fun CarryoverTheme(theme: String = "", wallpaper: Boolean = false, content: @Composable () -> Unit) {
    val dark = isDark(theme)
    val context = LocalContext.current
    val colors: ColorScheme = when {
        wallpaper && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S ->
            if (dark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        dark -> Dark
        else -> Light
    }
    CompositionLocalProvider(LocalStatusColors provides if (dark) DarkStatus else LightStatus) {
        MaterialTheme(colorScheme = colors, shapes = AppShapes, typography = AppTypography, content = content)
    }
}
