package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudQueue
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.PriorityHigh
import androidx.compose.material.icons.outlined.Schedule
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.JobSummary

/** Board colour for a status. Kept next to the card so the two never drift. */
@Composable
fun JobStatus.tone(): TlTone = when (this) {
    JobStatus.Intake -> TlTone.Info
    JobStatus.Diagnosed -> TlTone.Info
    JobStatus.WaitingParts -> TlTone.Warning
    JobStatus.InProgress -> TlTone.Info
    JobStatus.ReadyForPickup -> TlTone.Success
    JobStatus.Completed -> TlTone.Success
    JobStatus.Collected -> TlTone.Neutral
    JobStatus.Cancelled, JobStatus.Unrepairable -> TlTone.Danger
}

@Composable
fun JobStatusChip(status: JobStatus, modifier: Modifier = Modifier, short: Boolean = false) {
    TlStatusPill(
        text = if (short) status.shortLabel else status.label,
        tone = status.tone(),
        modifier = modifier,
    )
}

/**
 * One row on the board.
 *
 * Only what a technician scans for: who, what, where it is, when it's due. The
 * indicators are small on purpose — a card that shouts about everything tells
 * you nothing at a glance.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun JobCard(
    job: JobSummary,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    technicianName: String? = null,
) {
    TlCard(
        modifier = modifier,
        onClick = onClick,
        contentPadding = androidx.compose.foundation.layout.PaddingValues(TlTheme.spacing.lg),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xxs),
            ) {
                Text(
                    job.jobCode,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                )
                Text(
                    job.customerName ?: "Walk-in",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    job.deviceLabel,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            JobStatusChip(job.status)
        }

        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
        ) {
            if (job.isUrgent) {
                JobMeta(
                    icon = Icons.Outlined.PriorityHigh,
                    text = "Customer waiting",
                    tint = MaterialTheme.colorScheme.error,
                )
            }
            job.promisedBy?.let { due ->
                JobMeta(
                    icon = Icons.Outlined.Schedule,
                    text = formatDue(due),
                    tint = if (job.isOverdue) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            technicianName?.let {
                JobMeta(icon = null, text = it, tint = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (job.awaitingApproval) {
                TlStatusPill(text = "Approval pending", tone = TlTone.Warning, leadingDot = false)
            }
            if (job.partsPending) {
                JobMeta(
                    icon = Icons.Outlined.Inventory2,
                    text = "Parts required",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (job.pendingSync) {
                JobMeta(
                    icon = Icons.Outlined.CloudQueue,
                    text = "Not synced",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

@Composable
private fun JobMeta(icon: ImageVector?, text: String, tint: Color) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
    ) {
        if (icon != null) {
            Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(14.dp()))
        }
        Text(text, style = MaterialTheme.typography.labelMedium, color = tint, maxLines = 1)
    }
}

private fun Int.dp() = androidx.compose.ui.unit.Dp(this.toFloat())

/** Relative when it matters ("in 2h", "3h late"), absolute when it doesn't. */
internal fun formatDue(due: Long): String {
    val delta = due - System.currentTimeMillis()
    val minutes = kotlin.math.abs(delta) / 60_000
    val late = delta < 0
    return when {
        minutes < 60 -> if (late) "${minutes}m late" else "in ${minutes}m"
        minutes < 60 * 24 -> {
            val hours = minutes / 60
            if (late) "${hours}h late" else "in ${hours}h"
        }
        else -> {
            val days = minutes / (60 * 24)
            if (late) "${days}d late" else "in ${days}d"
        }
    }
}
