package com.techlane.pos.core.designsystem.component

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.theme.TlTheme

/**
 * Primary action. Full-height (56dp) by default so it stays hittable with a
 * thumb while the other hand holds a customer's phone.
 */
@Composable
fun TlButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
    large: Boolean = false,
    containerColor: Color = MaterialTheme.colorScheme.primary,
    contentColor: Color = MaterialTheme.colorScheme.onPrimary,
) {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    // 2% squeeze: enough to feel responsive on a cheap panel, too small to read as bounce.
    val scale by animateFloatAsState(if (pressed) 0.98f else 1f, label = "buttonPress")

    Button(
        onClick = onClick,
        modifier = modifier
            .scale(scale)
            .heightIn(min = if (large) TlTheme.sizes.controlHeightLarge else TlTheme.sizes.controlHeight),
        enabled = enabled && !loading,
        shape = MaterialTheme.shapes.small,
        interactionSource = interaction,
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = containerColor.copy(alpha = 0.35f),
            disabledContentColor = contentColor.copy(alpha = 0.6f),
        ),
        elevation = ButtonDefaults.buttonElevation(defaultElevation = 0.dp, pressedElevation = 0.dp),
        contentPadding = ButtonDefaults.ContentPadding,
    ) {
        ButtonContent(text = text, loading = loading, icon = icon, large = large)
    }
}

@Composable
fun TlSecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
) {
    OutlinedButton(
        onClick = onClick,
        modifier = modifier.heightIn(min = TlTheme.sizes.controlHeight),
        enabled = enabled && !loading,
        shape = MaterialTheme.shapes.small,
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
        colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.onSurface),
    ) {
        ButtonContent(text = text, loading = loading, icon = icon, large = false)
    }
}

/** Gold accent — reserved for the single "money" action on a screen. */
@Composable
fun TlAccentButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
    large: Boolean = true,
) = TlButton(
    text = text,
    onClick = onClick,
    modifier = modifier,
    enabled = enabled,
    loading = loading,
    icon = icon,
    large = large,
    containerColor = TlTheme.colors.accent,
    contentColor = TlTheme.colors.onAccent,
)

@Composable
fun TlDangerButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
) = TlButton(
    text = text,
    onClick = onClick,
    modifier = modifier,
    enabled = enabled,
    loading = loading,
    icon = icon,
    containerColor = MaterialTheme.colorScheme.error,
    contentColor = MaterialTheme.colorScheme.onError,
)

@Composable
fun TlTextButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    color: Color = MaterialTheme.colorScheme.primary,
) {
    TextButton(
        onClick = onClick,
        modifier = modifier.heightIn(min = TlTheme.sizes.minTouchTarget),
        enabled = enabled,
        shape = MaterialTheme.shapes.extraSmall,
    ) {
        Text(text, style = MaterialTheme.typography.labelLarge, color = color)
    }
}

@Composable
private fun ButtonContent(text: String, loading: Boolean, icon: ImageVector?, large: Boolean) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier.fillMaxWidth(),
    ) {
        when {
            loading -> CircularProgressIndicator(
                modifier = Modifier.size(TlTheme.sizes.icon),
                strokeWidth = 2.dp,
                color = androidx.compose.material3.LocalContentColor.current,
            )
            icon != null -> Icon(icon, contentDescription = null, modifier = Modifier.size(TlTheme.sizes.icon))
        }
        Text(
            text = text,
            style = if (large) MaterialTheme.typography.titleMedium else MaterialTheme.typography.labelLarge,
            maxLines = 1,
        )
    }
}
