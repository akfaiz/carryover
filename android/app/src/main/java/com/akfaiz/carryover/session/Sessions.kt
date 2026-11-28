package com.akfaiz.carryover.session

import android.content.Context
import com.akfaiz.carryover.R
import com.akfaiz.carryover.data.AndroidTexts
import com.akfaiz.carryover.data.error
import android.content.Intent
import com.akfaiz.carryover.core.Core
import com.akfaiz.carryover.core.EmulatorView
import com.akfaiz.carryover.core.Launch
import com.akfaiz.carryover.core.LaunchException
import com.akfaiz.carryover.core.Profile
import com.akfaiz.carryover.core.toIntent

/**
 * Starting a play session, for the Play button and for launches from
 * frontends and shortcuts (LaunchActivity): the Android parts (what starts a
 * profile, the service, the Intent) around [PlayFlow], which holds the rules.
 */
object Sessions {
    /** What starts a profile with its own setup on this device: the Intent and the app it starts. */
    data class Launchable(val intent: Intent, val pkg: String)

    /** The app setup of a profile, as (exe, args, rom), or null when none is set up. */
    fun setup(p: Profile, emulators: List<EmulatorView>): Triple<String, String, String>? {
        if (p.emulatorId == null) return p.app.takeIf { it.isNotBlank() }?.let { Triple(it, "", "") }
        val emu = emulators.firstOrNull { it.id == p.emulatorId } ?: return null
        return emu.device.exe.takeIf { it.isNotBlank() }?.let { Triple(it, emu.device.args, p.rom) }
    }

    /** The Intent that plays a profile with its own setup, or why it can't. */
    fun launchable(ctx: Context, p: Profile, emulators: List<EmulatorView>): Result<Launchable> {
        val (exe, args, rom) = setup(p, emulators)
            ?: return Result.failure(
                LaunchException(
                    if (p.emulator == null) {
                        ctx.getString(R.string.session_set_up_app)
                    } else {
                        ctx.getString(R.string.session_set_up_emulator, p.emulator)
                    },
                ),
            )
        return try {
            val spec = Launch.parse(exe, args, rom)
            val intent = spec.toIntent(ctx.packageManager)
                ?: return Result.failure(LaunchException(ctx.getString(R.string.session_not_installed, spec.pkg)))
            Result.success(Launchable(intent, spec.pkg))
        } catch (e: LaunchException) {
            Result.failure(e)
        }
    }

    private fun flow(ctx: Context) = PlayFlow(
        core = Core,
        manager = SessionManager.shared,
        watch = { key, pkg, title -> SessionService.watch(ctx, key, pkg, title) },
        stopWatching = { key -> SessionService.done(ctx, key) },
        startFailed = { title, cause -> ctx.getString(R.string.session_start_failed, title, AndroidTexts(ctx).error(cause)) },
    )

    /**
     * Plays profile key (native: the game's own profile) by starting intent,
     * whose app is pkg: [PlayFlow.play].
     */
    suspend fun play(ctx: Context, key: String, native: Boolean, title: String, intent: Intent, pkg: String): PlayOutcome =
        flow(ctx).play(key, native, title, pkg) { start(ctx, intent, title) }

    /** Syncs and watches a game that is already running: [PlayFlow.track]. */
    suspend fun track(ctx: Context, key: String, native: Boolean, title: String, pkg: String): PlayOutcome =
        flow(ctx).track(key, native, title, pkg)

    /** Starts the game; the reason it couldn't be started, or null. */
    private fun start(ctx: Context, intent: Intent, title: String): String? = try {
        ctx.startActivity(intent)
        null
    } catch (e: Exception) {
        ctx.getString(R.string.session_start_failed, title, e.message ?: e.toString())
    }
}
