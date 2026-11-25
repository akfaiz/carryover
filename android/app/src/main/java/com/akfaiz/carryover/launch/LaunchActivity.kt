package com.akfaiz.carryover.launch

import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.content.pm.ShortcutInfoCompat
import androidx.core.content.pm.ShortcutManagerCompat
import androidx.core.graphics.drawable.IconCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.AppLanguage
import com.akfaiz.carryover.core.AppPrefs
import com.akfaiz.carryover.core.Core
import com.akfaiz.carryover.data.AndroidTexts
import com.akfaiz.carryover.ui.AppButton
import com.akfaiz.carryover.ui.BtnKind
import com.akfaiz.carryover.ui.CarryoverTheme
import com.akfaiz.carryover.ui.PromptDialog
import com.akfaiz.carryover.ui.SectionCard
import com.akfaiz.carryover.ui.Spinner

/**
 * Starts a game for a frontend (ES-DE, Daijisho, …) or a home-screen
 * shortcut, with its saves synced: the frontend sends Carryover the intent
 * it would send the emulator, plus carryover.target naming the emulator;
 * a shortcut sends carryover.key naming the game profile. Carryover syncs,
 * starts the game and closes; the session watcher uploads after playing.
 * See docs/android-frontends.md.
 */
class LaunchActivity : ComponentActivity() {
    // Made for the intent that started this activity (the view model holds it through its platform).
    private val vm: LaunchViewModel by viewModels {
        viewModelFactory {
            initializer { LaunchViewModel(Core, AndroidLaunchPlatform(application, intent), AndroidTexts(application)) }
        }
    }

    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(AppLanguage.wrap(base))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        vm.start()
        setContent {
            CarryoverTheme(AppPrefs.theme(this), AppPrefs.wallpaperColours(this)) {
                LaunchScreen(vm) { finish() }
            }
        }
    }

    companion object {
        const val ACTION_PLAY = "com.akfaiz.carryover.PLAY"

        /**
         * Asks the launcher to pin a shortcut that plays profile key, with the
         * game's cover (coverPath, "" for none) as its icon; false when it can't.
         */
        fun pinShortcut(ctx: Context, key: String, title: String, coverPath: String = ""): Boolean {
            if (!ShortcutManagerCompat.isRequestPinShortcutSupported(ctx)) return false
            return ShortcutManagerCompat.requestPinShortcut(ctx, shortcut(ctx, key, title, coverPath), null)
        }

        /**
         * Puts profile key among the app icon's long-press shortcuts (the most
         * recently played first; the system drops the oldest beyond its limit).
         */
        fun pushRecent(ctx: Context, key: String, title: String, coverPath: String = "") {
            runCatching { ShortcutManagerCompat.pushDynamicShortcut(ctx, shortcut(ctx, key, title, coverPath)) }
        }

        /** The intent that starts watching profile key's game again, without starting the game (it is already running). */
        fun trackIntent(ctx: Context, key: String): Intent = Intent(ctx, LaunchActivity::class.java)
            .setAction(ACTION_PLAY)
            .putExtra(Forward.EXTRA_KEY, key)
            .putExtra(Forward.EXTRA_TRACK, true)

        private fun shortcut(ctx: Context, key: String, title: String, coverPath: String): ShortcutInfoCompat {
            val intent = Intent(ctx, LaunchActivity::class.java)
                .setAction(ACTION_PLAY)
                .putExtra(Forward.EXTRA_KEY, key)
            return ShortcutInfoCompat.Builder(ctx, "play-$key")
                .setShortLabel(title.take(25))
                .setLongLabel(title)
                .setIcon(icon(ctx, coverPath))
                .setIntent(intent)
                .build()
        }

        private const val ICON_PX = 192

        /** The cover cropped to a square, or the app's icon when the game has none. */
        private fun icon(ctx: Context, coverPath: String): IconCompat {
            if (coverPath.isNotEmpty()) {
                try {
                    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
                    BitmapFactory.decodeFile(coverPath, bounds)
                    val opts = BitmapFactory.Options().apply {
                        inSampleSize = maxOf(1, minOf(bounds.outWidth, bounds.outHeight) / ICON_PX)
                    }
                    val bmp = BitmapFactory.decodeFile(coverPath, opts)
                    if (bmp != null) {
                        val side = minOf(bmp.width, bmp.height)
                        val square = Bitmap.createBitmap(bmp, (bmp.width - side) / 2, 0, side, side)
                        return IconCompat.createWithBitmap(Bitmap.createScaledBitmap(square, ICON_PX, ICON_PX, true))
                    }
                } catch (_: Exception) {
                    // fall back to the app's icon
                } catch (_: OutOfMemoryError) {
                }
            }
            return IconCompat.createWithResource(ctx, R.mipmap.ic_launcher)
        }
    }
}

@Composable
private fun LaunchScreen(vm: LaunchViewModel, close: () -> Unit) {
    val state by vm.state.collectAsStateWithLifecycle()
    val prompts by vm.prompts.collectAsStateWithLifecycle()
    LaunchedEffect(state) { if (state is LaunchState.Done) close() }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.scrim.copy(alpha = 0.5f)), contentAlignment = Alignment.Center) {
        SectionCard(null, Modifier.padding(24.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                when (val s = state) {
                    is LaunchState.Working -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        Spinner()
                        Text(s.message)
                    }
                    is LaunchState.Choose -> Choose(s, vm, close)
                    is LaunchState.Error -> {
                        Text(stringResource(R.string.app_name), style = MaterialTheme.typography.titleMedium)
                        Text(s.message)
                        AppButton(onClick = close) { Text(stringResource(R.string.action_close)) }
                    }
                    LaunchState.Done -> {}
                }
            }
        }
    }
    prompts.firstOrNull()?.let { p -> PromptDialog(p) { vm.answer(p, it) } }
}

@Composable
private fun Choose(s: LaunchState.Choose, vm: LaunchViewModel, close: () -> Unit) {
    Text(stringResource(R.string.launch_which), style = MaterialTheme.typography.titleMedium)
    Text(
        if (s.file.isNotEmpty()) stringResource(R.string.launch_which_file, s.file, s.app) else stringResource(R.string.launch_which_app, s.app),
        style = MaterialTheme.typography.bodyMedium,
    )
    if (s.candidates.isEmpty()) {
        Text(stringResource(R.string.launch_no_candidates, s.app), style = MaterialTheme.typography.bodySmall)
    }
    Column(Modifier.heightIn(max = 320.dp).verticalScroll(rememberScrollState())) {
        for (c in s.candidates) {
            AppButton(onClick = { vm.choose(c) }, modifier = Modifier.fillMaxWidth(), kind = BtnKind.Ghost) { Text(c.title) }
        }
    }
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        AppButton(onClick = { vm.choose(null) }, kind = BtnKind.Secondary) { Text(stringResource(R.string.action_play_no_sync)) }
        AppButton(onClick = close, kind = BtnKind.Ghost) { Text(stringResource(R.string.action_cancel)) }
    }
}
