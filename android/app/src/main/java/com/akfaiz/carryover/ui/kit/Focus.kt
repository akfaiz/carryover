package com.akfaiz.carryover.ui.kit

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
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.unit.dp

/**
 * A visible ring while focused (D-pad, gamepad, keyboard): Material's own
 * focus highlight is too faint to follow from a sofa.
 */
fun Modifier.focusRing(shape: Shape? = null): Modifier = composed {
    val shape = shape ?: MaterialTheme.shapes.medium
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
