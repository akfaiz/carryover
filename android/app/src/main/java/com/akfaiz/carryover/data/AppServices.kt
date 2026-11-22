package com.akfaiz.carryover.data

import android.content.Context
import com.akfaiz.carryover.core.AppPrefs
import com.akfaiz.carryover.core.CoreApi

/** [PreferenceStore] on the app's SharedPreferences. */
class AndroidPreferenceStore(private val ctx: Context) : PreferenceStore {
    override val graceChoices: List<Int> get() = AppPrefs.graceChoices

    override var theme: String
        get() = AppPrefs.theme(ctx)
        set(v) = AppPrefs.setTheme(ctx, v)

    override var wallpaper: Boolean
        get() = AppPrefs.wallpaperColours(ctx)
        set(v) = AppPrefs.setWallpaperColours(ctx, v)

    override var graceSeconds: Int
        get() = AppPrefs.graceSeconds(ctx)
        set(v) = AppPrefs.setGraceSeconds(ctx, v)

    override var backgroundSync: Boolean
        get() = AppPrefs.backgroundSync(ctx)
        set(v) = AppPrefs.setBackgroundSync(ctx, v)
}

/**
 * What the view models are built from, set once by the Application. A view
 * model's constructor takes [shared] by default, which is what the framework's
 * factory uses; a test or a preview builds its own [AppServices] and passes it.
 */
class AppServices(
    val core: CoreApi,
    val settings: SettingsRepository,
    val messenger: Messenger,
    val platform: AppPlatform,
) {
    companion object {
        private var instance: AppServices? = null

        val shared: AppServices get() = checkNotNull(instance) { "AppServices not installed" }

        fun install(services: AppServices) {
            instance = services
        }
    }
}
