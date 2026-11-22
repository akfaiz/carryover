package com.akfaiz.carryover

import android.content.Context
import android.graphics.Color
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
import com.akfaiz.carryover.ui.AppViewModel
import com.akfaiz.carryover.ui.CarryoverApp
import com.akfaiz.carryover.ui.CarryoverTheme
import com.akfaiz.carryover.ui.SettingsViewModel
import com.akfaiz.carryover.ui.isDark

class MainActivity : ComponentActivity() {
    private val vm: AppViewModel by viewModels()
    private val settings: SettingsViewModel by viewModels()

    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(AppLanguage.wrap(base))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
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

    override fun onResume() {
        super.onResume()
        vm.onResume()
    }
}
