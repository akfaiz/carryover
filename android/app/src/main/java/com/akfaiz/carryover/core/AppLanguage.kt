package com.akfaiz.carryover.core

import android.app.Activity
import android.app.LocaleManager
import android.content.Context
import android.content.res.Configuration
import android.os.Build
import android.os.LocaleList
import java.util.Locale

/**
 * The app's language: the system's, or one the user chose in Settings.
 *
 * Android 13+ keeps a per-app language itself (LocaleManager; it also shows in
 * the system's App languages settings, from res/xml/locales_config.xml) and
 * restarts the activities. Before that, the choice is kept in AppPrefs and
 * applied by [wrap] in attachBaseContext of the Application, the activities and
 * the session service.
 */
object AppLanguage {
    /** The languages the app has text in, as language tags ("" = the system's). */
    val choices = listOf("", "en", "id")

    private val perApp: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU

    /** The chosen language tag, "" when the app follows the system. */
    fun current(ctx: Context): String {
        if (perApp) {
            val locales = ctx.getSystemService(LocaleManager::class.java).applicationLocales
            return if (locales.isEmpty) "" else normalize(locales[0].toLanguageTag())
        }
        return AppPrefs.language(ctx)
    }

    /** Switches the language; activity is recreated to show it. */
    fun set(activity: Activity, tag: String) {
        if (perApp) {
            // Android restarts the app's activities with the new locale.
            activity.getSystemService(LocaleManager::class.java).applicationLocales =
                if (tag.isEmpty()) LocaleList.getEmptyLocaleList() else LocaleList.forLanguageTags(tag)
            return
        }
        AppPrefs.setLanguage(activity, tag)
        @Suppress("DEPRECATION") // the Application's resources serve the ViewModels' messages
        activity.applicationContext.resources.updateConfiguration(
            configured(activity.applicationContext, tag),
            activity.applicationContext.resources.displayMetrics,
        )
        activity.recreate()
    }

    /** base with the chosen language applied (before Android 13; unchanged after). */
    fun wrap(base: Context): Context {
        if (perApp) return base
        val tag = AppPrefs.language(base)
        if (tag.isEmpty()) return base
        return base.createConfigurationContext(configured(base, tag))
    }

    private fun configured(ctx: Context, tag: String): Configuration {
        val config = Configuration(ctx.resources.configuration)
        val locale = if (tag.isEmpty()) LocaleList.getDefault()[0] else Locale.forLanguageTag(tag)
        config.setLocales(LocaleList(locale))
        return config
    }

    /** Indonesian is "in" in old Java locales, "id" in language tags: one value for both. */
    private fun normalize(tag: String): String = when {
        tag.startsWith("in") || tag.startsWith("id") -> "id"
        tag.startsWith("en") -> "en"
        else -> tag
    }
}
