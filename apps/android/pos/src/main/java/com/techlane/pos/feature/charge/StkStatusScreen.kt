package com.techlane.pos.feature.charge

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.automirrored.outlined.HelpOutline
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTextButton
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.MoneyDisplay
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.MpesaResult
import com.techlane.pos.domain.model.PaymentMethod
import com.techlane.pos.domain.model.StkStage

/**
 * The wait. Once a prompt is out, the counter must not be able to wander off,
 * start another charge, or guess — so this covers the screen and stays until the
 * payment is confirmed, refused, or explicitly parked for later.
 *
 * Nothing here ever *invents* an outcome: an unconfirmed prompt is shown as
 * unconfirmed, never as failed, because M-Pesa can still settle it afterwards.
 */
@Composable
fun StkStatusScreen(
    stage: StkStage,
    amount: Double,
    phone: String?,
    label: String,
    method: PaymentMethod,
    canForceReconcile: Boolean,
    receiptBusy: Boolean,
    receiptError: String?,
    onPrintReceipt: () -> Unit,
    onShareReceipt: () -> Unit,
    onRetry: () -> Unit,
    onTakeCash: () -> Unit,
    onCheckAgain: () -> Unit,
    onStopWaiting: () -> Unit,
    onDone: () -> Unit,
    onDismiss: () -> Unit,
) {
    val inFlight = stage is StkStage.Sending || stage is StkStage.Waiting || stage is StkStage.Finalising

    // Back must not escape a live prompt; it would leave a paid sale unattended.
    BackHandler(enabled = true) { if (!inFlight) onDismiss() }

    Dialog(
        onDismissRequest = { if (!inFlight) onDismiss() },
        properties = DialogProperties(
            dismissOnBackPress = false,
            dismissOnClickOutside = false,
            usePlatformDefaultWidth = false,
        ),
    ) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Box(Modifier.fillMaxSize().systemBarsPadding().padding(TlTheme.spacing.xxl)) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .widthIn(max = 460.dp)
                        .align(Alignment.Center),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
                ) {
                    AnimatedContent(
                        targetState = stage.artKey(),
                        transitionSpec = { (fadeIn(tween(220)) + scaleIn(tween(220), 0.9f)).togetherWith(fadeOut(tween(150))) },
                        label = "stkArt",
                    ) { key ->
                        when (key) {
                            ArtKey.Waiting -> PulsingPhone()
                            ArtKey.Success -> ResultGlyph(Icons.Outlined.CheckCircle, TlTheme.colors.success)
                            ArtKey.Failure -> ResultGlyph(Icons.Outlined.ErrorOutline, MaterialTheme.colorScheme.error)
                            ArtKey.Unknown -> ResultGlyph(Icons.AutoMirrored.Outlined.HelpOutline, TlTheme.colors.warning)
                        }
                    }

                    Text(
                        text = stage.headline(method),
                        style = MaterialTheme.typography.headlineSmall,
                        color = MaterialTheme.colorScheme.onBackground,
                        textAlign = TextAlign.Center,
                    )

                    Text(
                        text = formatKes(if (stage is StkStage.Paid) stage.amount else amount),
                        style = MoneyDisplay,
                        color = when (stage) {
                            is StkStage.Paid -> TlTheme.colors.success
                            is StkStage.Failed -> MaterialTheme.colorScheme.error
                            else -> MaterialTheme.colorScheme.onBackground
                        },
                    )

                    Text(
                        text = label,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center,
                    )

                    DetailBlock(
                        stage = stage,
                        phone = phone,
                        canForceReconcile = canForceReconcile,
                    )
                }

                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .widthIn(max = 460.dp)
                        .align(Alignment.BottomCenter),
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                ) {
                    TlBanner(message = receiptError, tone = TlTone.Danger)
                    Actions(
                        stage = stage,
                        receiptBusy = receiptBusy,
                        onPrintReceipt = onPrintReceipt,
                        onShareReceipt = onShareReceipt,
                        onRetry = onRetry,
                        onTakeCash = onTakeCash,
                        onCheckAgain = onCheckAgain,
                        onStopWaiting = onStopWaiting,
                        onDone = onDone,
                        onDismiss = onDismiss,
                    )
                }
            }
        }
    }
}

// ---------------------------------------------------------------- copy

private enum class ArtKey { Waiting, Success, Failure, Unknown }

private fun StkStage.artKey(): ArtKey = when (this) {
    StkStage.Sending, is StkStage.Waiting, StkStage.Finalising -> ArtKey.Waiting
    is StkStage.Paid -> ArtKey.Success
    is StkStage.Failed -> ArtKey.Failure
    is StkStage.TimedOut -> ArtKey.Unknown
}

private fun StkStage.headline(method: PaymentMethod): String = when (this) {
    StkStage.Sending -> if (method == PaymentMethod.Cash) "Recording cash…" else "Sending the prompt…"
    is StkStage.Waiting -> "Waiting for the customer"
    StkStage.Finalising -> "Payment received — closing the sale"
    is StkStage.Paid -> "Paid"
    is StkStage.Failed -> "Not paid"
    is StkStage.TimedOut -> "Not confirmed yet"
}

@Composable
private fun DetailBlock(stage: StkStage, phone: String?, canForceReconcile: Boolean) {
    val body: String? = when (stage) {
        StkStage.Sending -> phone?.let { "Sending to ${Msisdn.formatLocal(it)}" }
        is StkStage.Waiting -> stage.detail
        StkStage.Finalising -> "Deducting stock and filing the receipt."
        is StkStage.Paid -> stage.receipt?.let { "Reference $it" } ?: "The sale is closed and stock is updated."
        is StkStage.Failed -> stage.message
        is StkStage.TimedOut ->
            "M-Pesa hasn't told us either way yet. The customer may still complete it, " +
                "so check again before charging a second time." +
                if (canForceReconcile) "" else "\n\nAn owner or manager can force a check from the web console."
    }

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        if (stage is StkStage.Waiting) {
            Surface(shape = PillShape, color = MaterialTheme.colorScheme.surfaceVariant) {
                Text(
                    text = phone?.let { "Prompt on ${Msisdn.formatLocal(it)} · ${stage.secondsElapsed}s" }
                        ?: "${stage.secondsElapsed}s",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.sm),
                )
            }
        }
        if (body != null) {
            Text(
                text = body,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
        }
        if (stage is StkStage.Failed) {
            Text(
                text = MpesaResult.recovery(stage.reason),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun Actions(
    stage: StkStage,
    receiptBusy: Boolean,
    onPrintReceipt: () -> Unit,
    onShareReceipt: () -> Unit,
    onRetry: () -> Unit,
    onTakeCash: () -> Unit,
    onCheckAgain: () -> Unit,
    onStopWaiting: () -> Unit,
    onDone: () -> Unit,
    onDismiss: () -> Unit,
) {
    when (stage) {
        StkStage.Sending, StkStage.Finalising -> Unit

        is StkStage.Waiting -> {
            // Only offered once the prompt has had a fair chance — before that,
            // an escape hatch just invites double-charging.
            if (stage.secondsElapsed >= 20) {
                TlTextButton(
                    text = "I'll check this later",
                    onClick = onStopWaiting,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }

        is StkStage.Paid -> {
            // A receipt only exists once the sale is closed server-side, which is
            // exactly what reaching Paid means — so it is offered here and nowhere
            // earlier in the flow.
            val hasSale = stage.saleId != null
            if (hasSale) {
                TlButton(
                    text = if (receiptBusy) "Loading receipt…" else "Print receipt",
                    onClick = onPrintReceipt,
                    icon = Icons.Outlined.Print,
                    loading = receiptBusy,
                    modifier = Modifier.fillMaxWidth(),
                )
                TlSecondaryButton(
                    text = "Send receipt",
                    onClick = onShareReceipt,
                    icon = Icons.Outlined.Share,
                    enabled = !receiptBusy,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            TlButton(
                text = "New charge",
                onClick = onDone,
                large = !hasSale,
                containerColor = if (hasSale) {
                    MaterialTheme.colorScheme.surfaceVariant
                } else {
                    MaterialTheme.colorScheme.primary
                },
                contentColor = if (hasSale) {
                    MaterialTheme.colorScheme.onSurfaceVariant
                } else {
                    MaterialTheme.colorScheme.onPrimary
                },
                modifier = Modifier.fillMaxWidth(),
            )
        }

        is StkStage.Failed -> {
            TlButton(text = "Send the prompt again", onClick = onRetry, modifier = Modifier.fillMaxWidth())
            TlSecondaryButton(text = "Take cash instead", onClick = onTakeCash, modifier = Modifier.fillMaxWidth())
            TlTextButton(text = "Close", onClick = onDismiss, modifier = Modifier.fillMaxWidth())
        }

        is StkStage.TimedOut -> {
            TlButton(text = "Check again", onClick = onCheckAgain, modifier = Modifier.fillMaxWidth())
            TlSecondaryButton(text = "Take cash instead", onClick = onTakeCash, modifier = Modifier.fillMaxWidth())
            TlTextButton(text = "Close", onClick = onDismiss, modifier = Modifier.fillMaxWidth())
        }
    }
}

// ---------------------------------------------------------------- art

/** A slow sweep around a handset — reads as "their phone is ringing", not "loading". */
@Composable
private fun PulsingPhone() {
    val transition = rememberInfiniteTransition(label = "pulse")
    val sweep by transition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(tween(2200, easing = LinearEasing)),
        label = "sweep",
    )
    val breath by transition.animateFloat(
        initialValue = 0.85f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(1100, easing = LinearEasing), RepeatMode.Reverse),
        label = "breath",
    )
    val ring = MaterialTheme.colorScheme.primary
    val track = TlTheme.colors.hairline

    Box(contentAlignment = Alignment.Center, modifier = Modifier.size(132.dp)) {
        Canvas(Modifier.fillMaxSize()) {
            val stroke = 6.dp.toPx()
            val inset = stroke / 2
            val arcSize = Size(size.width - stroke, size.height - stroke)
            drawArc(
                color = track,
                startAngle = 0f,
                sweepAngle = 360f,
                useCenter = false,
                topLeft = Offset(inset, inset),
                size = arcSize,
                style = Stroke(width = stroke, cap = StrokeCap.Round),
            )
            drawArc(
                color = ring.copy(alpha = breath),
                startAngle = sweep,
                sweepAngle = 90f,
                useCenter = false,
                topLeft = Offset(inset, inset),
                size = arcSize,
                style = Stroke(width = stroke, cap = StrokeCap.Round),
            )
        }
        Icon(
            Icons.Outlined.PhoneAndroid,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.primary,
            modifier = Modifier.size(48.dp),
        )
    }
}

@Composable
private fun ResultGlyph(icon: ImageVector, tint: Color) {
    Box(contentAlignment = Alignment.Center, modifier = Modifier.size(132.dp)) {
        Surface(shape = PillShape, color = tint.copy(alpha = 0.12f), modifier = Modifier.size(112.dp)) {
            Box(contentAlignment = Alignment.Center) {
                Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(56.dp))
            }
        }
    }
}
