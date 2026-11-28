package com.akfaiz.carryover

import android.content.Context
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.drawable.ColorDrawable
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.akfaiz.carryover.core.AppLanguage
import com.akfaiz.carryover.core.AppPrefs
import com.akfaiz.carryover.ui.CarryoverApp
import com.akfaiz.carryover.ui.kit.CarryoverTheme
import com.akfaiz.carryover.ui.kit.isDark
import com.akfaiz.carryover.ui.settings.SettingsViewModel
import com.akfaiz.carryover.ui.state.AppViewModel

class MainActivity : ComponentActivity() {
    private val vm: AppViewModel by viewModels()
    private val settings: SettingsViewModel by viewModels()

    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(AppLanguage.wrap(base))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        paintWindowBackground()
        enableEdgeToEdge()
        setContent {
            val state by settings.state.collectAsStateWithLifecycle()
            val dark = isDark(state.theme)
            // The status and navigation bar icons follow the app's theme, not the system's.
            LaunchedEffect(dark) {
                val style = if (dark) {
                    SystemBarStyle.dark(Color.TRANSPARENT)
                } else {
                    SystemBarStyle.light(Color.TRANSPARENT, Color.TRANSPARENT)
                }
                enableEdgeToEdge(statusBarStyle = style, navigationBarStyle = style)
            }
            CarryoverTheme(state.theme, state.wallpaper) {
                CarryoverApp(vm)
            }
        }
    }

    /**
     * The window before Compose draws, in the app's own theme: the XML background only
     * follows the system's night mode, so a theme chosen in Settings would flash the other one.
     */
    private fun paintWindowBackground() {
        val dark = when (AppPrefs.theme(this)) {
            "dark" -> true
            "light" -> false
            else -> (resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES
        }
        window.setBackgroundDrawable(ColorDrawable(if (dark) 0xFF0E0F14.toInt() else 0xFFF5F6F9.toInt()))
    }

    override fun onResume() {
        super.onResume()
        vm.onResume()
    }
}
