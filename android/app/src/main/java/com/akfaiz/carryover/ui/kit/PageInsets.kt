package com.akfaiz.carryover.ui.kit

import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.only
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf

/**
 * The window insets a page's Scaffold pads its content by: the shell's
 * navigation bar or rail already keeps clear of the system bars on its side.
 */
val LocalPageInsets = staticCompositionLocalOf { WindowInsets(0, 0, 0, 0) }

/** A page's top bar insets: the shell's, so the rail's side isn't padded twice. */
@Composable
fun pageBarInsets(): WindowInsets = LocalPageInsets.current.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal)
