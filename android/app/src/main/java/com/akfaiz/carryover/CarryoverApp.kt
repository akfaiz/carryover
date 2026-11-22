package com.akfaiz.carryover

import android.app.Application
import android.content.Context
import android.util.Log
import com.akfaiz.carryover.core.AppLanguage
import com.akfaiz.carryover.core.AppPrefs
import com.akfaiz.carryover.core.Core
import com.akfaiz.carryover.data.AndroidPreferenceStore
import com.akfaiz.carryover.data.AndroidTexts
import com.akfaiz.carryover.data.AppServices
import com.akfaiz.carryover.data.Messenger
import com.akfaiz.carryover.data.SettingsRepository
import com.akfaiz.carryover.session.SessionManager
import com.akfaiz.carryover.session.SessionNotifier
import com.akfaiz.carryover.session.UsageFrontSource
import com.akfaiz.carryover.work.BackgroundSync
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class CarryoverApp : Application() {
    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(AppLanguage.wrap(base))
    }

    override fun onCreate() {
        super.onCreate()
        Core.init(this)
        SessionManager.install(
            SessionManager(
                core = Core,
                notices = SessionNotifier(this),
                front = UsageFrontSource(this),
                scope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
                graceMs = { AppPrefs.graceMs(this) },
                retryUploads = { BackgroundSync.retryUploads(this) },
                log = { Log.i("CarryoverSession", it) },
            ),
        )
        AppServices.install(
            AppServices(
                core = Core,
                settings = SettingsRepository(Core, AndroidPreferenceStore(this)) { BackgroundSync.schedule(this) },
                messenger = Messenger(AndroidTexts(this)),
                platform = AndroidPlatform(this),
            ),
        )
        BackgroundSync.schedule(this) // the "Sync in the background" setting
    }
}
