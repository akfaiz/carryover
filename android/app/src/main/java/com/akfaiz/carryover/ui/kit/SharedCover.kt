package com.akfaiz.carryover.ui.kit

import androidx.compose.animation.AnimatedVisibilityScope
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.Shape

/** Where covers fly between pages; null outside the app's pages (a preview, the launch screen). */
@OptIn(ExperimentalSharedTransitionApi::class)
val LocalSharedTransition = staticCompositionLocalOf<SharedTransitionScope?> { null }

/** The page being drawn, as it comes or goes; null outside the app's pages. */
val LocalPageScope = compositionLocalOf<AnimatedVisibilityScope?> { null }

/**
 * Marks a game's cover. When one page showing it gives way to another showing it, the cover grows or shrinks from
 * its place on the one to its place on the other. One page may show a game's cover only once. [shape] is how the
 * cover is clipped, which the clips around it can't do while it flies.
 */
@OptIn(ExperimentalSharedTransitionApi::class)
fun Modifier.sharedCover(gameId: String, shape: Shape): Modifier = composed {
    val shared = LocalSharedTransition.current
    val page = LocalPageScope.current
    if (shared == null || page == null) return@composed Modifier
    with(shared) {
        Modifier.sharedElement(
            rememberSharedContentState("cover:$gameId"),
            page,
            clipInOverlayDuringTransition = OverlayClip(shape),
        )
    }
}
