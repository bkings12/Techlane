package com.techlane.pos.feature.jobs

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.JobFilter
import com.techlane.pos.feature.jobs.components.JobCard
import com.techlane.pos.feature.jobs.components.SyncStatusIndicator

/**
 * The board. Compact status chips rather than tabs, because a technician needs
 * eight queues reachable without a menu, and tabs would eat a third of the screen.
 */
@Composable
fun JobsScreen(
    onOpenJob: (String) -> Unit,
    onScan: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: JobsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val jobs by viewModel.visibleJobs.collectAsStateWithLifecycle()
    val technicianNames by viewModel.technicianNames.collectAsStateWithLifecycle()

    TlScreen(
        title = "Jobs",
        subtitle = "${jobs.size} ${if (jobs.size == 1) "job" else "jobs"} · ${state.filter.label}",
        modifier = modifier,
        onRefresh = { viewModel.refresh() },
        refreshing = state.refreshing,
        actions = {
            IconButton(onClick = onScan) {
                Icon(Icons.Outlined.QrCodeScanner, contentDescription = "Scan a job")
            }
        },
    ) {
        TlTextField(
            value = state.query,
            onValueChange = viewModel::setQuery,
            label = "Search",
            placeholder = "Job number, customer, phone, model, IMEI",
            leadingIcon = Icons.Outlined.Search,
            showClear = true,
        )

        FilterChips(current = state.filter, onSelect = viewModel::setFilter)

        SyncStatusIndicator(pendingCount = state.pendingSync)

        TlBanner(
            message = state.error?.let { "$it — showing the last synced board." },
            tone = TlTone.Warning,
        )

        when {
            state.loading && jobs.isEmpty() -> repeat(4) { JobCardSkeleton() }

            jobs.isEmpty() -> TlEmptyState(
                title = emptyTitle(state.filter, state.query),
                subtitle = emptySubtitle(state.filter, state.query),
                icon = Icons.Outlined.Build,
            )

            else -> jobs.forEach { job ->
                JobCard(
                    job = job,
                    technicianName = job.technicianId?.let { technicianNames[it] },
                    onClick = { onOpenJob(job.id) },
                )
            }
        }
    }
}

@Composable
private fun FilterChips(current: JobFilter, onSelect: (JobFilter) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        JobFilter.entries.forEach { filter ->
            val selected = filter == current
            Surface(
                onClick = { onSelect(filter) },
                shape = RoundedCornerShape(10.dp),
                color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface,
                border = if (selected) {
                    null
                } else {
                    androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline)
                },
            ) {
                Text(
                    filter.label,
                    style = MaterialTheme.typography.labelMedium,
                    color = if (selected) {
                        MaterialTheme.colorScheme.onPrimary
                    } else {
                        MaterialTheme.colorScheme.onSurfaceVariant
                    },
                    modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = 10.dp),
                )
            }
        }
    }
}

/** Skeleton rather than a spinner — the board's shape is the useful hint. */
@Composable
private fun JobCardSkeleton() {
    Surface(
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceVariant,
        modifier = Modifier.fillMaxWidth().height(112.dp),
    ) {}
}

private fun emptyTitle(filter: JobFilter, query: String): String = when {
    query.isNotBlank() -> "Nothing matches \"$query\""
    filter == JobFilter.Mine -> "No jobs assigned to you"
    filter == JobFilter.All -> "No jobs on the board"
    else -> "Nothing in ${filter.label.lowercase()}"
}

private fun emptySubtitle(filter: JobFilter, query: String): String? = when {
    query.isNotBlank() -> "Try a job number, phone number or IMEI."
    filter == JobFilter.Mine -> "Jobs assigned to you will appear here. Switch to All to see the whole shop."
    filter == JobFilter.All -> "Pull down to refresh once the shop has taken something in."
    else -> null
}
