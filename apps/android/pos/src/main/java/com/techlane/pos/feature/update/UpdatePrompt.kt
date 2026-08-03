package com.techlane.pos.feature.update

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.SystemUpdateAlt
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlNeutralButton
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.data.update.AvailableUpdate

/**
 * "A newer TechLane is out." Shown once per version unless the server marks it
 * mandatory, in which case there is no way past it — that flag is the only
 * thing that can hold a shop out of its own till, so it is honoured literally
 * and never inferred locally.
 */
@Composable
fun UpdatePrompt(
    update: AvailableUpdate,
    installedVersionName: String,
    onDismiss: () -> Unit,
) {
    val uriHandler = LocalUriHandler.current
    val mandatory = update.mandatory

    // A dismissible prompt should honour Back; a mandatory one must not be
    // escapable by the one gesture every Android user tries first.
    BackHandler(enabled = mandatory) { /* deliberately inert */ }

    Dialog(
        onDismissRequest = { if (!mandatory) onDismiss() },
        properties = DialogProperties(
            dismissOnBackPress = !mandatory,
            dismissOnClickOutside = !mandatory,
        ),
    ) {
        Surface(
            shape = MaterialTheme.shapes.medium,
            color = MaterialTheme.colorScheme.surface,
            modifier = Modifier.widthIn(max = 400.dp),
        ) {
            Column(
                modifier = Modifier.padding(TlTheme.spacing.xl),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
            ) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Surface(
                        shape = MaterialTheme.shapes.small,
                        color = MaterialTheme.colorScheme.primaryContainer,
                        modifier = Modifier.size(44.dp),
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Outlined.SystemUpdateAlt,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                                modifier = Modifier.size(TlTheme.sizes.icon),
                            )
                        }
                    }
                    Column {
                        Text(
                            if (mandatory) "Update required" else "Update available",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                        )
                        Text(
                            "TechLane ${update.versionName}",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }

                Text(
                    if (mandatory) {
                        "This release has to be installed before the till can be used again."
                    } else {
                        "You're on $installedVersionName. Installing takes about a minute."
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                val bullets = update.noteLines
                if (bullets.isNotEmpty()) {
                    Text(
                        "What's new",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    bullets.take(4).forEach { line ->
                        Text(
                            "•  $line",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface,
                        )
                    }
                }

                update.downloadUrl?.let { url ->
                    TlButton(
                        text = "Update now",
                        onClick = { runCatching { uriHandler.openUri(url) } },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                if (!mandatory) {
                    TlNeutralButton(text = "Later", onClick = onDismiss, modifier = Modifier.fillMaxWidth())
                }
            }
        }
    }
}
