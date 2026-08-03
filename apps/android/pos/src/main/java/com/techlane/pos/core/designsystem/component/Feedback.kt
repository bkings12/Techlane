package com.techlane.pos.core.designsystem.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme

enum class TlTone { Neutral, Info, Success, Warning, Danger }

/** Foreground/background pair for a tone, resolved against the active scheme. */
@Composable
fun toneColors(tone: TlTone): Pair<Color, Color> = when (tone) {
    TlTone.Neutral -> MaterialTheme.colorScheme.onSurfaceVariant to MaterialTheme.colorScheme.surfaceVariant
    TlTone.Info -> TlTheme.colors.onInfoContainer to TlTheme.colors.infoContainer
    TlTone.Success -> TlTheme.colors.onSuccessContainer to TlTheme.colors.successContainer
    TlTone.Warning -> TlTheme.colors.onWarningContainer to TlTheme.colors.warningContainer
    TlTone.Danger -> MaterialTheme.colorScheme.onErrorContainer to MaterialTheme.colorScheme.errorContainer
}

private fun toneIcon(tone: TlTone): ImageVector = when (tone) {
    TlTone.Success -> Icons.Outlined.CheckCircle
    TlTone.Warning -> Icons.Outlined.WarningAmber
    TlTone.Danger -> Icons.Outlined.ErrorOutline
    else -> Icons.Outlined.Info
}

/** Inline message strip. Collapses out of layout when [message] is null. */
@Composable
fun TlBanner(
    message: String?,
    modifier: Modifier = Modifier,
    tone: TlTone = TlTone.Danger,
    action: (@Composable () -> Unit)? = null,
) {
    AnimatedVisibility(
        visible = !message.isNullOrBlank(),
        enter = fadeIn() + expandVertically(),
        exit = fadeOut() + shrinkVertically(),
        modifier = modifier,
    ) {
        val (fg, bg) = toneColors(tone)
        Surface(shape = MaterialTheme.shapes.small, color = bg, modifier = Modifier.fillMaxWidth()) {
            Row(
                modifier = Modifier.padding(TlTheme.spacing.md),
                horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(toneIcon(tone), contentDescription = null, tint = fg, modifier = Modifier.size(TlTheme.sizes.icon))
                Text(
                    text = message.orEmpty(),
                    style = MaterialTheme.typography.bodyMedium,
                    color = fg,
                    modifier = Modifier.weight(1f),
                )
                action?.invoke()
            }
        }
    }
}

/** Compact status chip — repair states, payment states, stock states. */
@Composable
fun TlStatusPill(
    text: String,
    modifier: Modifier = Modifier,
    tone: TlTone = TlTone.Neutral,
    leadingDot: Boolean = true,
) {
    val (fg, bg) = toneColors(tone)
    Surface(shape = PillShape, color = bg, modifier = modifier) {
        Row(
            modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = TlTheme.spacing.xs + 2.dp),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs + 2.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (leadingDot) {
                Box(modifier = Modifier.size(7.dp).background(fg, PillShape))
            }
            Text(text, style = MaterialTheme.typography.labelSmall, color = fg)
        }
    }
}

@Composable
fun TlEmptyState(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    icon: ImageVector? = null,
    action: (@Composable () -> Unit)? = null,
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = TlTheme.spacing.xxl, vertical = TlTheme.spacing.huge),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
    ) {
        if (icon != null) {
            Surface(
                shape = PillShape,
                color = MaterialTheme.colorScheme.surfaceVariant,
                modifier = Modifier.size(64.dp),
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        icon,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(TlTheme.sizes.iconLg),
                    )
                }
            }
        }
        Text(
            title,
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface,
            textAlign = TextAlign.Center,
        )
        if (subtitle != null) {
            Text(
                subtitle,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
        }
        action?.invoke()
    }
}

@Composable
fun TlLoading(modifier: Modifier = Modifier, label: String? = null) {
    Column(
        modifier = modifier.fillMaxWidth().padding(TlTheme.spacing.xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
    ) {
        CircularProgressIndicator(strokeWidth = 3.dp, color = MaterialTheme.colorScheme.primary)
        if (label != null) {
            Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

/** Blocks the whole screen while an irreversible action is in flight. */
@Composable
fun TlBlockingOverlay(visible: Boolean, label: String, modifier: Modifier = Modifier) {
    AnimatedVisibility(visible = visible, enter = fadeIn(), exit = fadeOut(), modifier = modifier) {
        Box(
            modifier = Modifier.fillMaxSize().background(TlTheme.colors.scrimHeavy),
            contentAlignment = Alignment.Center,
        ) {
            TlLoading(label = label)
        }
    }
}
