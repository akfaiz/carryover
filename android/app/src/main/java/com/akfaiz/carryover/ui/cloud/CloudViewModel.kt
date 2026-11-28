package com.akfaiz.carryover.ui.cloud

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.akfaiz.carryover.R
import com.akfaiz.carryover.core.Provider
import com.akfaiz.carryover.core.Question
import com.akfaiz.carryover.data.AppServices
import com.akfaiz.carryover.data.SettingsState
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

/** The cloud pages: the accounts, choosing the one to sync with, and connecting a new one. */
class CloudViewModel(private val services: AppServices = AppServices.shared) : ViewModel() {

    private val repo get() = services.settings
    private val core get() = services.core
    private val m get() = services.messenger

    val state: StateFlow<SettingsState> get() = repo.state

    /** The sign-in page of the provider being connected (null when none). */
    val authUrl: StateFlow<String?> get() = core.authUrl

    fun clearAuthUrl() = core.clearAuthUrl()
    fun openBrowser(url: String) = core.openBrowser(url)

    fun refresh() {
        viewModelScope.launch { m.attempt(m.str(R.string.err_read_settings)) { repo.refresh() } }
    }

    suspend fun providers(): List<Provider> = core.providers()

    /** First step of connecting a provider; the answer is rclone's next question (or done). */
    suspend fun createRemote(name: String, type: String, params: Map<String, String>): Question =
        core.createRemote(name, type, params)

    suspend fun continueRemote(name: String, state: String, answer: String): Question =
        core.continueRemote(name, state, answer)

    /** Gives up waiting for the browser sign-in: the step that waits for it fails and its account is dropped. */
    fun cancelSignIn() {
        viewModelScope.launch { runCatching { core.cancelSignIn() } }
    }

    /**
     * Copies the accounts of the supported kinds from the contents of an rclone.conf the user picked, and says what
     * happened in one message: how many, which ones replaced an account of the same name, which were left out.
     */
    fun importConfig(contents: String) {
        viewModelScope.launch {
            val res = m.attempt(m.str(R.string.err_import_config)) { core.importRcloneConfig(contents) } ?: return@launch
            val parts = mutableListOf<String>()
            parts += if (res.imported.isEmpty()) {
                m.str(R.string.msg_import_none)
            } else {
                m.plural(R.plurals.msg_import_done, res.imported.size, res.imported.size)
            }
            if (res.replaced.isNotEmpty()) parts += m.str(R.string.msg_import_replaced, res.replaced.joinToString(", "))
            if (res.skipped.isNotEmpty()) parts += m.str(R.string.msg_import_skipped, res.skipped.joinToString(", "))
            m.say(parts.joinToString(" "))
            m.attempt(m.str(R.string.err_read_settings)) { repo.refresh() }
        }
    }

    /** Says the file the user picked could not be read. */
    fun importUnreadable(e: Exception) = m.failed(m.str(R.string.err_import_config), e)

    /** Syncs with [remote]:[root]; [done] gets whether it worked (the page stays open on a failure). */
    fun useCloud(remote: String, root: String, done: (Boolean) -> Unit = {}) {
        viewModelScope.launch {
            val ok = m.attempt(m.str(R.string.err_use_cloud, "$remote:$root")) {
                repo.useCloud(remote, root)
                m.say(m.str(R.string.msg_cloud_connected))
            } != null
            done(ok)
        }
    }

    fun deleteRemote(name: String) {
        viewModelScope.launch {
            val ok = m.attempt(m.str(R.string.err_delete_remote, name)) { repo.deleteRemote(name) } != null
            if (ok) m.say(m.str(R.string.msg_remote_deleted, name))
            m.attempt(m.str(R.string.err_read_settings)) { repo.refresh() }
        }
    }
}
