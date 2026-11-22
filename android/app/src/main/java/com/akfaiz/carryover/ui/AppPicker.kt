package com.akfaiz.carryover.ui

import android.content.Context
import android.content.Intent
import android.graphics.drawable.Drawable
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.draw.clip
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.akfaiz.carryover.R
import androidx.compose.ui.unit.dp
import androidx.core.graphics.drawable.toBitmap
import com.akfaiz.carryover.core.EmulatorPresets
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** An installed app with a launcher screen. */
data class InstalledApp(val pkg: String, val label: String, val icon: Drawable?) {
    val knownEmulator: String? get() = EmulatorPresets.forPackage(pkg)?.name
}

/** Installed apps that can be opened, known emulators first, then by name; this app left out. */
suspend fun launchableApps(ctx: Context): List<InstalledApp> = withContext(Dispatchers.IO) {
    val pm = ctx.packageManager
    val found = mutableMapOf<String, InstalledApp>()
    for (category in listOf(Intent.CATEGORY_LAUNCHER, Intent.CATEGORY_LEANBACK_LAUNCHER)) {
        val intent = Intent(Intent.ACTION_MAIN).addCategory(category)
        for (ri in pm.queryIntentActivities(intent, 0)) {
            val pkg = ri.activityInfo.packageName
            if (pkg == ctx.packageName || pkg in found) continue
            found[pkg] = InstalledApp(pkg, ri.loadLabel(pm).toString(), runCatching { ri.loadIcon(pm) }.getOrNull())
        }
    }
    found.values.sortedWith(compareBy({ it.knownEmulator == null }, { it.label.lowercase() }))
}

/** Lets the user pick an installed app. */
@Composable
fun AppPickerDialog(title: String, onDismiss: () -> Unit, onPick: (InstalledApp) -> Unit) {
    val ctx = LocalContext.current
    var apps by remember { mutableStateOf<List<InstalledApp>?>(null) }
    var query by remember { mutableStateOf("") }
    LaunchedEffect(Unit) { apps = launchableApps(ctx) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(query, { query = it }, label = { Text(stringResource(R.string.action_search)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                val list = apps
                if (list == null) {
                    CircularProgressIndicator()
                } else {
                    val shown = list.filter {
                        query.isBlank() || it.label.contains(query, ignoreCase = true) || it.pkg.contains(query, ignoreCase = true)
                    }
                    LazyColumn(Modifier.heightIn(max = 420.dp)) {
                        items(shown, key = { it.pkg }) { app ->
                            Row(
                                Modifier.fillMaxWidth().focusRing().clip(RoundedCornerShape(14.dp)).clickable { onPick(app) }.padding(8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(12.dp),
                            ) {
                                app.icon?.let {
                                    Image(it.toBitmap(96, 96).asImageBitmap(), contentDescription = null, Modifier.size(36.dp))
                                }
                                Column {
                                    Text(app.label)
                                    Text(
                                        app.knownEmulator?.let { "${app.pkg} · $it" } ?: app.pkg,
                                        style = MaterialTheme.typography.bodySmall,
                                    )
                                }
                            }
                        }
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_cancel)) } },
    )
}

/** An app's label, or its package when it isn't installed. */
fun appLabel(ctx: Context, exe: String): String {
    val pkg = exe.substringBefore('/')
    return runCatching {
        val pm = ctx.packageManager
        pm.getApplicationLabel(pm.getApplicationInfo(pkg, 0)).toString()
    }.getOrDefault(pkg)
}
