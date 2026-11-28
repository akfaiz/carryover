package com.akfaiz.carryover.ui.kit

import androidx.compose.animation.AnimatedContentTransitionScope
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.SizeTransform
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.ProgressIndicatorDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.IntOffset

/*
 * How the app moves, in one place, as ui/kit/motion.go does for the desktop. It is short, so that nothing waits on
 * it. Compose's animations follow Android's animator duration scale, so a user who turns animations off sees none.
 */

/** Something small changing, or on its way out. */
const val MotionFast = 120

/** Content coming in. */
const val MotionBase = 200

/**
 * One thing giving way to another in the same place: the new one fades in over the old one fading out. What they take
 * up resizes with them, without clipping a focus ring.
 */
fun <S> AnimatedContentTransitionScope<S>.crossOver(): ContentTransform =
    fadeIn(tween(MotionBase)) togetherWith fadeOut(tween(MotionFast)) using SizeTransform(clip = false)

/**
 * What is on its way out takes no taps, and no presses of a gamepad's or a keyboard's select button: it is still
 * drawn, showing what was so a moment ago, and must not act on it.
 */
fun Modifier.inert(): Modifier = this
    .pointerInput(Unit) {
        awaitPointerEventScope {
            while (true) awaitPointerEvent(PointerEventPass.Initial).changes.forEach { it.consume() }
        }
    }
    .onPreviewKeyEvent { it.key == Key.DirectionCenter || it.key == Key.Enter || it.key == Key.NumPadEnter || it.key == Key.ButtonA }

/** A bar that slides to each new [progress] (0 to 1) instead of jumping there. */
@Composable
fun AppProgressBar(progress: Float, modifier: Modifier = Modifier) {
    val shown by animateFloatAsState(progress, ProgressIndicatorDefaults.ProgressAnimationSpec, label = "progress")
    LinearProgressIndicator(progress = { shown }, modifier = modifier)
}

/**
 * How an item of a lazy list moves to its place (`animateItem`'s placement): in
 * step with a card that grows, which takes the same time on the same curve.
 * Inside the pages' SharedTransitionLayout a lazy list puts its items straight
 * where they will end up, so every item below such a card needs this: without
 * it they jump there while the card is on its way, and any slower motion draws
 * them over the card.
 */
val PlacementMotion: FiniteAnimationSpec<IntOffset> = tween(MotionBase)

/** A row a list draws: one of its own, or one that just left it and is on its way out. */
data class ShownRow<T>(val key: Any, val item: T, val leaving: Boolean = false)

/**
 * The rows to draw when a list that drew [shown] is now [items]: those, in their order, with the ones that just left
 * kept where they were, marked [ShownRow.leaving]. A row that comes back stops leaving.
 */
fun <T> mergeRows(shown: List<ShownRow<T>>, items: List<T>, key: (T) -> Any): List<ShownRow<T>> {
    val rows = items.mapTo(mutableListOf()) { ShownRow(key(it), it) }
    var after = 0 // where the next row on its way out goes: behind the one it followed
    for (old in shown) {
        val at = rows.indexOfFirst { it.key == old.key }
        if (at >= 0) after = at + 1 else rows.add(after++, old.copy(leaving = true))
    }
    return rows
}
