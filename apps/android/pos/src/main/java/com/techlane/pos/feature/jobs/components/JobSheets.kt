package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowForward
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlDangerButton
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlNeutralButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.data.local.TechnicianEntity
import com.techlane.pos.domain.model.ClosureReason
import com.techlane.pos.domain.model.JobStatus

/**
 * Status picker.
 *
 * Only transitions the server actually allows from the current status are shown
 * (see JobStatus.allowedNext, mirrored from internal/repair/status.go) — an
 * option that always fails is worse than no option. Statuses whose consequences
 * are hard to undo ask for a second tap in place, rather than a dialog.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StatusPickerBottomSheet(
    current: JobStatus,
    blockedReason: String?,
    onSelect: (JobStatus, note: String?, closureReason: ClosureReason?) -> Unit,
    onRequestApproval: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var confirming by remember { mutableStateOf<JobStatus?>(null) }
    var note by remember { mutableStateOf("") }
    var closureReason by remember { mutableStateOf<ClosureReason?>(null) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.xl)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text("Update status", style = MaterialTheme.typography.titleLarge)
            Text(
                "Currently ${current.label}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            val pending = confirming
            if (pending == null) {
                current.allowedNext.forEach { next ->
                    val blocked = next == JobStatus.InProgress && blockedReason != null
                    StatusOption(
                        status = next,
                        blocked = blocked,
                        onClick = {
                            when {
                                blocked -> confirming = next
                                next.needsConfirmation -> confirming = next
                                else -> onSelect(next, null, null)
                            }
                        },
                    )
                }
                if (current.allowedNext.isEmpty()) {
                    Text(
                        "This job is finished — there is nothing further to move it to.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            } else if (pending == JobStatus.InProgress && blockedReason != null) {
                // The authorization gate. Deliberately not bypassable: the server
                // refuses this transition too, so pretending otherwise here would
                // only produce a failure the technician cannot explain.
                TlBanner(message = blockedReason, tone = TlTone.Warning)
                Text(
                    "Repair work cannot begin until the customer has agreed to the price.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                TlButton(
                    text = "Send estimate or record approval",
                    onClick = {
                        onRequestApproval()
                        onDismiss()
                    },
                    modifier = Modifier.fillMaxWidth(),
                )
                TlNeutralButton(
                    text = "Back",
                    onClick = { confirming = null },
                    modifier = Modifier.fillMaxWidth(),
                )
            } else {
                Text("Confirm: ${pending.label}", style = MaterialTheme.typography.titleMedium)
                if (pending.isClosure) {
                    Text(
                        "Closing a job writes off its value, so it needs a reason.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    ClosureReason.forStatus(pending).forEach { reason ->
                        SelectableRow(
                            label = reason.label,
                            selected = closureReason == reason,
                            onClick = { closureReason = reason },
                        )
                    }
                }
                TlTextField(
                    value = note,
                    onValueChange = { note = it },
                    label = "Note (optional)",
                    placeholder = "What changed?",
                    singleLine = false,
                )
                if (pending.isClosure) {
                    TlDangerButton(
                        text = "Mark ${pending.label.lowercase()}",
                        onClick = { onSelect(pending, note.takeIf { it.isNotBlank() }, closureReason) },
                        enabled = closureReason != null,
                        modifier = Modifier.fillMaxWidth(),
                    )
                } else {
                    TlButton(
                        text = "Confirm ${pending.label.lowercase()}",
                        onClick = { onSelect(pending, note.takeIf { it.isNotBlank() }, null) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                TlNeutralButton(
                    text = "Back",
                    onClick = { confirming = null },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

@Composable
private fun StatusOption(status: JobStatus, blocked: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.surfaceVariant,
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 64.dp)
                .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            JobStatusChip(status)
            Column(modifier = Modifier.weight(1f)) {
                Text(status.label, style = MaterialTheme.typography.titleSmall)
                if (blocked) {
                    Text(
                        "Needs customer approval",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
            }
            Icon(
                Icons.AutoMirrored.Outlined.ArrowForward,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun SelectableRow(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.extraSmall,
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
        border = androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.bodyMedium,
            modifier = Modifier.padding(TlTheme.spacing.md),
        )
    }
}

/** Searchable technician list — shops with twenty benches should not scroll. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TechnicianPicker(
    technicians: List<TechnicianEntity>,
    currentId: String?,
    meId: String?,
    onSelect: (TechnicianEntity) -> Unit,
    onAssignToMe: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var query by remember { mutableStateOf("") }
    val filtered = remember(technicians, query) {
        if (query.isBlank()) technicians
        else technicians.filter { it.displayName.contains(query, ignoreCase = true) }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.xl)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text(
                if (currentId == null) "Assign technician" else "Reassign technician",
                style = MaterialTheme.typography.titleLarge,
            )

            if (meId != null && currentId != meId) {
                TlButton(text = "Assign to me", onClick = onAssignToMe, modifier = Modifier.fillMaxWidth())
            }

            if (technicians.size > 6) {
                TlTextField(
                    value = query,
                    onValueChange = { query = it },
                    label = "Search",
                    placeholder = "Technician name",
                    leadingIcon = Icons.Outlined.Search,
                    showClear = true,
                )
            }

            LazyColumn(
                modifier = Modifier.heightIn(max = 360.dp),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
            ) {
                items(filtered, key = { it.id }) { tech ->
                    Surface(
                        onClick = { onSelect(tech) },
                        modifier = Modifier.fillMaxWidth(),
                        color = MaterialTheme.colorScheme.surface,
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(vertical = TlTheme.spacing.md),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                        ) {
                            Surface(
                                shape = PillShape,
                                color = MaterialTheme.colorScheme.primary.copy(alpha = 0.12f),
                                modifier = Modifier.size(36.dp),
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(
                                        Icons.Outlined.Person,
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.primary,
                                        modifier = Modifier.size(18.dp),
                                    )
                                }
                            }
                            Text(
                                tech.displayName,
                                style = MaterialTheme.typography.titleSmall,
                                modifier = Modifier.weight(1f),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (tech.id == currentId) {
                                Text(
                                    "Assigned",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.primary,
                                )
                            }
                        }
                    }
                    TlDivider()
                }
                if (filtered.isEmpty()) {
                    item {
                        Text(
                            "No technicians match that.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(vertical = TlTheme.spacing.lg),
                        )
                    }
                }
            }
        }
    }
}
