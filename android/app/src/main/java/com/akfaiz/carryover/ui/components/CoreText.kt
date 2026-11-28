package com.akfaiz.carryover.ui.components

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import com.akfaiz.carryover.core.CoreMessage
import com.akfaiz.carryover.core.GameView
import com.akfaiz.carryover.data.AndroidTexts
import com.akfaiz.carryover.data.Texts
import com.akfaiz.carryover.data.core

/** The app's texts as the screen shows them, in its current language. */
@Composable
fun rememberTexts(): Texts {
    val context = LocalContext.current
    val configuration = LocalConfiguration.current
    return remember(context, configuration) { AndroidTexts(context) }
}

/** A field of the core ([english] with its [message]) in the app's language. */
@Composable
fun coreText(english: String, message: CoreMessage?): String = rememberTexts().core(english, message)
