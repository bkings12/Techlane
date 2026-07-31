package com.techlane.core.update

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.techlane.core.theme.Brand
import kotlinx.coroutines.launch

/**
 * Settings / More-sheet control: shows the installed version and a
 * "Check for updates" action that opens the download URL when a newer build exists.
 */
@Composable
fun AppUpdateSettingsPanel(
    apiBase: String,
    appKey: String,
    currentVersionCode: Int,
    currentVersionName: String,
    modifier: Modifier = Modifier,
) {
    var busy by remember { mutableStateOf(false) }
    var info by remember { mutableStateOf<UpdateInfo?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val uriHandler = LocalUriHandler.current

    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text("App update", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Bold, color = Brand.Navy)
        Text(
            "Installed v$currentVersionName ($currentVersionCode)",
            style = MaterialTheme.typography.bodySmall,
            color = Brand.TextSecondary,
        )
        message?.let {
            Text(
                it,
                style = MaterialTheme.typography.bodySmall,
                color = if (info?.updateAvailable == true) Brand.Navy else Brand.TextSecondary,
            )
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedButton(
                onClick = {
                    busy = true
                    message = null
                    scope.launch {
                        val result = UpdateChecker.check(apiBase, appKey, currentVersionCode)
                        info = result
                        message = when {
                            result == null -> "Could not reach the update server. Try again online."
                            result.updateAvailable -> "Version ${result.latestVersionName} is available."
                            else -> "You're on the latest version."
                        }
                        busy = false
                    }
                },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                if (busy) {
                    CircularProgressIndicator(modifier = Modifier.height(18.dp).width(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(8.dp))
                }
                Text(if (busy) "Checking…" else "Check for updates")
            }
            if (info?.updateAvailable == true && !info?.downloadUrl.isNullOrBlank()) {
                Button(
                    onClick = {
                        info?.downloadUrl?.let { url -> runCatching { uriHandler.openUri(url) } }
                    },
                    modifier = Modifier.weight(1f),
                ) {
                    Text("Update")
                }
            }
        }
        info?.notes?.takeIf { it.isNotBlank() && info?.updateAvailable == true }?.let {
            Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.TextMuted)
        }
    }
}
