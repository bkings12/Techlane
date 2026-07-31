package com.techlane.core.update

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import com.techlane.core.ui.UpdateAvailableBanner

/**
 * Wraps the app's root content with a launch-time update-availability check.
 * Never blocks or delays rendering of [content] — the check runs
 * asynchronously and silently no-ops on failure (offline, old backend, etc).
 * Mount this once, directly under `TechLaneTheme`, in each app's MainActivity.
 *
 * Manual checks also live in More → App update ([AppUpdateSettingsPanel]).
 */
@Composable
fun AppUpdateGate(
    apiBase: String,
    appKey: String,
    currentVersionCode: Int,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    var info by remember { mutableStateOf<UpdateInfo?>(null) }
    var dismissed by remember { mutableStateOf(false) }
    val uriHandler = LocalUriHandler.current

    LaunchedEffect(appKey, currentVersionCode) {
        info = UpdateChecker.check(apiBase, appKey, currentVersionCode)
        // Fresh install of a new build clears any prior dismiss for this session.
        dismissed = false
    }

    Column(modifier.fillMaxSize()) {
        val current = info
        if (current != null && current.updateAvailable && (current.forceUpdate || !dismissed)) {
            UpdateAvailableBanner(
                versionName = current.latestVersionName,
                forceUpdate = current.forceUpdate,
                onUpdateClick = {
                    current.downloadUrl?.let { url -> runCatching { uriHandler.openUri(url) } }
                },
                onDismiss = if (current.forceUpdate) null else ({ dismissed = true }),
            )
        }
        content()
    }
}
