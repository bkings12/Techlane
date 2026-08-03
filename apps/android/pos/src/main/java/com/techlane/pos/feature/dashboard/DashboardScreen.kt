package com.techlane.pos.feature.dashboard

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowForward
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.AddCircleOutline
import androidx.compose.material.icons.outlined.NotificationsNone
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.AttentionItem
import com.techlane.pos.domain.model.AttentionLevel
import com.techlane.pos.domain.model.AttentionTarget
import com.techlane.pos.domain.model.DashboardTile
import com.techlane.pos.domain.model.JobFilter
import com.techlane.pos.domain.model.JobSummary
import com.techlane.pos.domain.model.QuickAction
import com.techlane.pos.domain.model.RecentActivity
import com.techlane.pos.feature.jobs.components.JobStatusChip
import com.techlane.pos.feature.jobs.components.SyncStatusIndicator
import com.techlane.pos.feature.jobs.components.formatDue
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * The shop floor's first screen.
 *
 * Ordered by what a technician needs to decide: what is on fire, what is mine,
 * what can I start right now. Counts are queues, not analytics — every number
 * and every row opens the thing it is counting.
 */
@Composable
fun DashboardScreen(
    onOpenJobs: (JobFilter) -> Unit,
    onOpenJob: (String) -> Unit,
    onScan: () -> Unit,
    onNewSale: () -> Unit,
    onNewIntake: () -> Unit,
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: DashboardViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val data by viewModel.data.collectAsStateWithLifecycle()

    TlScreen(
        title = "${greetingFor()}, ${state.firstName}",
        subtitle = state.prefs.branchName ?: "TechLane",
        modifier = modifier,
        onRefresh = { viewModel.refresh() },
        refreshing = state.refreshing,
        actions = {
            IconButton(onClick = { onOpenJobs(JobFilter.All) }) {
                Icon(Icons.Outlined.NotificationsNone, contentDescription = "Notifications")
            }
            IconButton(onClick = onOpenSettings) {
                Surface(
                    shape = PillShape,
                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.14f),
                    modifier = Modifier.size(30.dp),
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Text(
                            state.firstName.take(1).uppercase(),
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.primary,
                        )
                    }
                }
            }
        },
    ) {
        TlBanner(
            message = state.error?.let { "$it — showing the last synced figures." },
            tone = TlTone.Warning,
        )

        // Only surfaced when there is something queued; a permanent "all synced"
        // card would cost a row of screen to say nothing.
        SyncStatusIndicator(pendingCount = data.pendingSync)

        QuickActionsRow(
            actions = state.quickActions,
            onAction = { action ->
                when (action) {
                    QuickAction.Scan -> onScan()
                    QuickAction.FindJob -> onOpenJobs(JobFilter.All)
                    QuickAction.NewSale -> onNewSale()
                    QuickAction.NewIntake -> onNewIntake()
                }
            },
        )

        if (state.loading && data.isEmpty) {
            repeat(3) { DashboardSkeleton() }
            return@TlScreen
        }

        SummaryTiles(tiles = data.summary.tiles(), onOpen = onOpenJobs)

        if (data.attention.isNotEmpty()) {
            SectionHeading("Needs attention")
            TlCard(contentPadding = PaddingValues(vertical = TlTheme.spacing.xs)) {
                data.attention.forEachIndexed { index, item ->
                    AttentionRow(
                        item = item,
                        onClick = {
                            when (val target = item.target) {
                                is AttentionTarget.Board -> onOpenJobs(target.filter)
                                is AttentionTarget.Job -> onOpenJob(target.jobId)
                                AttentionTarget.Sync -> onOpenSettings()
                            }
                        },
                    )
                    if (index != data.attention.lastIndex) TlDivider()
                }
            }
        }

        if (data.myJobs.isNotEmpty()) {
            SectionHeading("My jobs", action = "View all" to { onOpenJobs(JobFilter.Mine) })
            TlCard(contentPadding = PaddingValues(vertical = TlTheme.spacing.xs)) {
                data.myJobs.forEachIndexed { index, job ->
                    MyJobRow(job = job, onClick = { onOpenJob(job.id) })
                    if (index != data.myJobs.lastIndex) TlDivider()
                }
            }
        }

        if (data.activity.isNotEmpty()) {
            SectionHeading("Recent activity")
            TlCard(contentPadding = PaddingValues(vertical = TlTheme.spacing.xs)) {
                data.activity.forEachIndexed { index, entry ->
                    ActivityRow(entry = entry, onClick = { entry.jobId?.let(onOpenJob) })
                    if (index != data.activity.lastIndex) TlDivider()
                }
            }
        }

        if (data.isEmpty && !state.loading) {
            TlCard {
                Text("Nothing on the board yet", style = MaterialTheme.typography.titleSmall)
                Text(
                    "Once the shop takes a device in, your queues and recent activity show up here.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        Box(Modifier.height(TlTheme.spacing.sm))
    }
}

@Composable
private fun SectionHeading(text: String, action: Pair<String, () -> Unit>? = null) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text.uppercase(),
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        action?.let { (label, onClick) ->
            Surface(onClick = onClick, color = Color.Transparent, shape = MaterialTheme.shapes.extraSmall) {
                Text(
                    label,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 4.dp),
                )
            }
        }
    }
}

/** Horizontal strip, not a grid: five queues should not own half the screen. */
@Composable
private fun SummaryTiles(tiles: List<DashboardTile>, onOpen: (JobFilter) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        tiles.forEach { tile ->
            Surface(
                onClick = { onOpen(tile.filter) },
                shape = MaterialTheme.shapes.small,
                color = MaterialTheme.colorScheme.surface,
                border = androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline),
                modifier = Modifier.width(118.dp),
            ) {
                Column(
                    modifier = Modifier.padding(TlTheme.spacing.md),
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                ) {
                    Text(
                        tile.count.toString(),
                        style = MaterialTheme.typography.headlineMedium,
                        color = if (tile.count == 0) {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        } else {
                            MaterialTheme.colorScheme.onSurface
                        },
                    )
                    Text(
                        tile.label,
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

@Composable
private fun AttentionRow(item: AttentionItem, onClick: () -> Unit) {
    // Colour is earned: only genuinely costly states get the alarm treatment.
    val tint = when (item.level) {
        AttentionLevel.Urgent -> MaterialTheme.colorScheme.error
        AttentionLevel.Warning -> TlTheme.colors.warning
        AttentionLevel.Info -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(onClick = onClick, color = Color.Transparent, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Surface(shape = PillShape, color = tint.copy(alpha = 0.14f), modifier = Modifier.size(30.dp)) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        item.count.toString(),
                        style = MaterialTheme.typography.labelMedium,
                        color = tint,
                    )
                }
            }
            Text(
                item.label,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Icon(
                Icons.AutoMirrored.Outlined.ArrowForward,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(16.dp),
            )
        }
    }
}

@Composable
private fun MyJobRow(job: JobSummary, onClick: () -> Unit) {
    Surface(onClick = onClick, color = Color.Transparent, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    job.deviceLabel,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    listOfNotNull(job.customerName, job.jobCode).joinToString(" · "),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                job.promisedBy?.let { due ->
                    Text(
                        formatDue(due),
                        style = MaterialTheme.typography.labelSmall,
                        color = if (job.isOverdue) {
                            MaterialTheme.colorScheme.error
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        },
                    )
                }
            }
            JobStatusChip(job.status, short = true)
        }
    }
}

@Composable
private fun ActivityRow(entry: RecentActivity, onClick: () -> Unit) {
    Surface(onClick = onClick, color = Color.Transparent, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.sm + 2.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text(
                timeFormat.format(Date(entry.at)),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.width(60.dp),
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(entry.title, style = MaterialTheme.typography.bodyMedium, maxLines = 1)
                entry.detail?.let {
                    Text(
                        it,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

@Composable
private fun QuickActionsRow(actions: List<QuickAction>, onAction: (QuickAction) -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        actions.forEach { action ->
            Surface(
                onClick = { onAction(action) },
                shape = MaterialTheme.shapes.small,
                color = MaterialTheme.colorScheme.surfaceVariant,
                modifier = Modifier.weight(1f),
            ) {
                Column(
                    modifier = Modifier.padding(vertical = TlTheme.spacing.md),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                ) {
                    Icon(
                        action.icon(),
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(20.dp),
                    )
                    Text(
                        action.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                    )
                }
            }
        }
    }
}

private fun QuickAction.icon(): ImageVector = when (this) {
    QuickAction.Scan -> Icons.Outlined.QrCodeScanner
    QuickAction.FindJob -> Icons.Outlined.Search
    QuickAction.NewSale -> Icons.AutoMirrored.Outlined.ReceiptLong
    QuickAction.NewIntake -> Icons.Outlined.AddCircleOutline
}

/** Shape-of-the-content placeholder; no full-screen spinner once cached. */
@Composable
private fun DashboardSkeleton() {
    Surface(
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceVariant,
        modifier = Modifier.fillMaxWidth().height(96.dp),
    ) {}
}

private val timeFormat = SimpleDateFormat("h:mm a", Locale.getDefault())
