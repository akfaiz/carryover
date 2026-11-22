package com.akfaiz.carryover.core

import android.content.Context
import android.content.SharedPreferences

/**
 * Preferences of the Android app itself, read before the Go core is open:
 * the theme (also kept in the core's Settings.Theme, as on the desktop; cached
 * here for the first frame), wallpaper colours and, before Android 13, the
 * language.
 */
object AppPrefs {
    private const val FILE = "carryover"
    private const val THEME = "theme"
    private const val WALLPAPER = "wallpaperColours"
    private const val LANGUAGE = "language"
    private const val GRACE = "graceSeconds"
    private const val BACKGROUND_SYNC = "backgroundSync"

    private fun prefs(ctx: Context): SharedPreferences = ctx.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    /** "" (follow the system), "light" or "dark": the values of the core's Settings.Theme. */
    fun theme(ctx: Context): String = prefs(ctx).getString(THEME, "").orEmpty()
    fun setTheme(ctx: Context, theme: String) = prefs(ctx).edit().putString(THEME, theme).apply()

    fun wallpaperColours(ctx: Context): Boolean = prefs(ctx).getBoolean(WALLPAPER, false)
    fun setWallpaperColours(ctx: Context, on: Boolean) = prefs(ctx).edit().putBoolean(WALLPAPER, on).apply()

    /** Seconds a game may be out of front before its session ends (see session/Front.kt). */
    val graceChoices = listOf(10, 30, 60)

    fun graceSeconds(ctx: Context): Int = prefs(ctx).getInt(GRACE, graceChoices.first()).takeIf { it in graceChoices } ?: graceChoices.first()
    fun graceMs(ctx: Context): Long = graceSeconds(ctx) * 1000L
    fun setGraceSeconds(ctx: Context, seconds: Int) = prefs(ctx).edit().putInt(GRACE, seconds).apply()

    /** Sync all games now and then while the device is charging, idle and on an unmetered network. */
    fun backgroundSync(ctx: Context): Boolean = prefs(ctx).getBoolean(BACKGROUND_SYNC, false)
    fun setBackgroundSync(ctx: Context, on: Boolean) = prefs(ctx).edit().putBoolean(BACKGROUND_SYNC, on).apply()

    /** A BCP 47 language tag ("en", "id") or "" for the system's language (before Android 13). */
    fun language(ctx: Context): String = prefs(ctx).getString(LANGUAGE, "").orEmpty()
    fun setLanguage(ctx: Context, tag: String) = prefs(ctx).edit().putString(LANGUAGE, tag).commit()
}
