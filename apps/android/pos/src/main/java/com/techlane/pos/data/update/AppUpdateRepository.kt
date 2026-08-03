package com.techlane.pos.data.update

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.techlane.pos.BuildConfig
import com.techlane.pos.data.remote.TechLaneApi
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import javax.inject.Inject
import javax.inject.Singleton

private val Context.updateStore by preferencesDataStore(name = "techlane_pos_updates")

/** A newer build the server has told us about. */
data class AvailableUpdate(
    val versionCode: Int,
    val versionName: String,
    val downloadUrl: String?,
    val notes: String?,
    /** Server-flagged: the app should stop being usable until it is updated. */
    val mandatory: Boolean,
) {
    /** Release notes split into bullets — the backend sends free text. */
    val noteLines: List<String>
        get() = notes.orEmpty()
            .split('\n', '•', ';')
            .map { it.trim().trimStart('-', '*').trim() }
            .filter { it.isNotBlank() }
}

/**
 * Owns "is there a newer APK". TechLane distributes its own APK, so this is
 * the only channel staff have to learn a build exists.
 *
 * Deliberately never downloads or installs anything — [AvailableUpdate.downloadUrl]
 * is handed to the browser. Keeping the decision to install with the operator
 * is both an Android platform constraint for sideloaded APKs and the safer
 * default for a till that might be mid-transaction.
 */
@Singleton
class AppUpdateRepository @Inject constructor(
    @ApplicationContext private val context: Context,
    private val api: TechLaneApi,
) {
    private val store = context.updateStore

    private val _available = MutableStateFlow<AvailableUpdate?>(null)

    /** The newest update found this process, or null if none/not checked yet. */
    val available: Flow<AvailableUpdate?> = _available.asStateFlow()

    val installedVersionName: String = BuildConfig.VERSION_NAME
    val installedVersionCode: Int = BuildConfig.VERSION_CODE

    /**
     * True when there is an update the operator has not already dismissed.
     * Mandatory updates ignore dismissal entirely — that is the whole point
     * of the flag, and the server is the only thing allowed to set it.
     */
    val promptable: Flow<AvailableUpdate?> = combine(_available, store.data) { update, prefs ->
        if (update == null) return@combine null
        val dismissed = prefs[KEY_DISMISSED_VERSION] ?: 0
        when {
            update.mandatory -> update
            update.versionCode > dismissed -> update
            else -> null
        }
    }

    /**
     * Asks the server, at most once per [MIN_CHECK_INTERVAL_MS] unless [force].
     * Failures are silent by design: an unreachable update server must never
     * be something a technician at a counter has to think about.
     */
    suspend fun check(force: Boolean = false): AvailableUpdate? {
        if (!force && !isDue()) return _available.value
        val result = runCatching {
            api.appVersion(
                app = APP_KEY,
                platform = "android",
                currentVersionCode = installedVersionCode,
            )
        }.getOrNull()
        // Only a successful check updates the timestamp, so a flaky connection
        // doesn't buy the server a full interval of silence.
        markChecked()
        if (result == null) return _available.value

        _available.value = if (result.updateAvailable && result.latestVersionCode > installedVersionCode) {
            AvailableUpdate(
                versionCode = result.latestVersionCode,
                versionName = result.latestVersionName,
                downloadUrl = result.downloadUrl,
                notes = result.notes,
                mandatory = result.forceUpdate,
            )
        } else {
            null
        }
        return _available.value
    }

    /** Hides the prompt for this version. The Settings badge stays. */
    suspend fun dismiss(versionCode: Int) {
        store.edit { it[KEY_DISMISSED_VERSION] = versionCode }
    }

    private suspend fun isDue(): Boolean {
        val last = store.data.first()[KEY_LAST_CHECK] ?: 0L
        return System.currentTimeMillis() - last >= MIN_CHECK_INTERVAL_MS
    }

    private suspend fun markChecked() {
        store.edit { it[KEY_LAST_CHECK] = System.currentTimeMillis() }
    }

    private companion object {
        /** Distinct from "ops"/"customer" — each app has its own release track. */
        const val APP_KEY = "pos"

        /**
         * Six hours. Long enough that reopening the app all day costs one
         * request, short enough that a shop picks up a same-day fix.
         */
        const val MIN_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000L

        val KEY_LAST_CHECK = longPreferencesKey("last_check_at")
        val KEY_DISMISSED_VERSION = intPreferencesKey("dismissed_version_code")
    }
}
