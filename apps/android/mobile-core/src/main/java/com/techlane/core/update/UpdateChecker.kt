package com.techlane.core.update

import com.techlane.core.network.ApiHttp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** Result of a GET /app-version check against the platform API. */
data class UpdateInfo(
    val updateAvailable: Boolean,
    val forceUpdate: Boolean,
    val latestVersionName: String,
    val downloadUrl: String?,
    val notes: String?,
)

/**
 * Lightweight, dependency-free "is a newer build available?" check used by
 * all three Android apps on launch. Silently returns null on any failure
 * (offline, backend not reachable, endpoint missing) — an update banner is
 * a nice-to-have, never something that should block or crash app startup.
 */
object UpdateChecker {
    suspend fun check(apiBase: String, app: String, currentVersionCode: Int): UpdateInfo? =
        withContext(Dispatchers.IO) {
            runCatching {
                val http = ApiHttp(apiBase, tokenProvider = { null })
                val obj = http.get("/app-version?app=$app&platform=android&current_version_code=$currentVersionCode")
                UpdateInfo(
                    updateAvailable = obj.optBoolean("update_available", false),
                    forceUpdate = obj.optBoolean("force_update", false),
                    latestVersionName = obj.optString("latest_version_name", ""),
                    downloadUrl = obj.optString("download_url").takeIf { it.isNotBlank() },
                    notes = obj.optString("notes").takeIf { it.isNotBlank() },
                )
            }.getOrNull()
        }
}
