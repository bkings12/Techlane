package com.techlane.pos.feature.intake

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlNeutralButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme

/**
 * What the counter sees the moment a job exists.
 *
 * Deliberately not a return to the board: the operator's next move is almost
 * always to hand over paper, so the receipt actions lead. The job is already
 * saved by the time this appears — nothing here can fail in a way that loses it.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun IntakeSuccessSheet(
    success: IntakeSuccess,
    onPrint: () -> Unit,
    onOpenJob: () -> Unit,
    onNewIntake: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.gutter)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Surface(shape = PillShape, color = TlTheme.colors.successContainer, modifier = Modifier.size(44.dp)) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Outlined.CheckCircle,
                            contentDescription = null,
                            tint = TlTheme.colors.success,
                            modifier = Modifier.size(TlTheme.sizes.icon),
                        )
                    }
                }
                Column {
                    Text("Job created", style = MaterialTheme.typography.titleMedium)
                    Text(
                        success.jobCode.ifBlank { "Saved" },
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            Text(success.deviceLabel, style = MaterialTheme.typography.titleSmall)
            Text(
                success.customerName,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            // The receipt's own state, separate from the job's. A printer that
            // is off is worth saying out loud, but it is not a failed intake.
            success.receiptStatus?.let { status ->
                Surface(
                    shape = MaterialTheme.shapes.small,
                    color = MaterialTheme.colorScheme.surfaceVariant,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text(
                        status,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(TlTheme.spacing.md),
                    )
                }
            }

            TlButton(
                text = "Print receipt",
                onClick = onPrint,
                icon = Icons.Outlined.Print,
                modifier = Modifier.fillMaxWidth(),
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
            ) {
                TlSecondaryButton(text = "Open job", onClick = onOpenJob, modifier = Modifier.weight(1f))
                TlNeutralButton(text = "New intake", onClick = onNewIntake, modifier = Modifier.weight(1f))
            }
        }
    }
}
