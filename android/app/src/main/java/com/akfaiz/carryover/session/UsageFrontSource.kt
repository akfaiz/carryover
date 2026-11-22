package com.akfaiz.carryover.session

import android.app.usage.UsageEvents
import android.app.usage.UsageStatsManager
import android.content.Context
import com.akfaiz.carryover.core.hasUsageAccess

/** [FrontSource] from Android's usage statistics (the user's Usage access). */
class UsageFrontSource(private val ctx: Context) : FrontSource {
    override fun hasUsageAccess(): Boolean = hasUsageAccess(ctx)

    override fun events(from: Long, to: Long): List<FrontEvent> {
        val usm = ctx.getSystemService(Context.USAGE_STATS_SERVICE) as UsageStatsManager
        val events = usm.queryEvents(from, to) ?: return emptyList()
        val e = UsageEvents.Event()
        val out = mutableListOf<FrontEvent>()
        while (events.hasNextEvent()) {
            events.getNextEvent(e)
            if (e.eventType == UsageEvents.Event.ACTIVITY_RESUMED) out += FrontEvent(e.packageName, e.timeStamp)
        }
        return out
    }
}
