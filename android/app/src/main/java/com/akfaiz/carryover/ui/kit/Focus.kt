package com.akfaiz.carryover.ui.kit

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.border
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.unit.dp

/**
 * A visible ring while focused (D-pad, gamepad, keyboard): Material's own
 * focus highlight is too faint to follow from a sofa. It fades from the control
 * the focus leaves to the one it reaches.
 */
fun Modifier.focusRing(shape: Shape? = null): Modifier = composed {
    val shape = shape ?: MaterialTheme.shapes.medium
    var focused by remember { mutableStateOf(false) }
    val primary = MaterialTheme.colorScheme.primary
    val ring by animateColorAsState(if (focused) primary else primary.copy(alpha = 0f), tween(MotionFast), label = "focusRing")
    this
        .onFocusChanged { focused = it.hasFocus }
        .then(if (ring.alpha > 0f) Modifier.border(3.dp, ring, shape) else Modifier)
}

/** How much the focused tile of a grid grows: little enough to stay within the room around it. */
private const val FocusedScale = 1.04f

/**
 * Raises a tile while the D-pad or the gamepad is on it, so what is picked
 * shows from the couch. A touch focuses nothing, so it does nothing then. Put
 * it first, so that the ring and the content grow together.
 */
fun Modifier.focusLift(): Modifier = composed {
    var focused by remember { mutableStateOf(false) }
    val scale = animateFloatAsState(
        if (focused) FocusedScale else 1f,
        spring(stiffness = Spring.StiffnessMediumLow),
        label = "focusLift",
    )
    this
        .onFocusChanged { focused = it.hasFocus }
        // Read as the layer draws, so a frame of the motion recomposes nothing.
        .graphicsLayer {
            scaleX = scale.value
            scaleY = scale.value
        }
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
