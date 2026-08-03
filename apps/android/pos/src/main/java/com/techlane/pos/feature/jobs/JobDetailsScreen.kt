package com.techlane.pos.feature.jobs

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Call
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlKeyValue
import com.techlane.pos.core.designsystem.component.TlLoading
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.JobAction
import com.techlane.pos.domain.model.JobDetail
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.feature.jobs.components.ApprovalCard
import com.techlane.pos.feature.jobs.components.CustomerUpdateSheet
import com.techlane.pos.feature.jobs.components.DiagnosisSheet
import com.techlane.pos.feature.jobs.components.EstimateSheet
import com.techlane.pos.feature.jobs.components.JobStatusChip
import com.techlane.pos.feature.jobs.components.PartsCard
import com.techlane.pos.feature.jobs.components.PartsPickerSheet
import com.techlane.pos.feature.jobs.components.PhotoKindSheet
import com.techlane.pos.feature.jobs.components.RecordApprovalSheet
import com.techlane.pos.feature.jobs.components.RepairPhotoGallery
import com.techlane.pos.feature.jobs.components.RepairTimeline
import com.techlane.pos.feature.jobs.components.StatusPickerBottomSheet
import com.techlane.pos.feature.jobs.components.SyncStatusIndicator
import com.techlane.pos.feature.jobs.components.TechnicianPicker
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * The technician's workspace for one repair.
 *
 * Everything routine happens here without leaving the screen: status, diagnosis,
 * approval, parts and photos are all bottom sheets, which is what keeps a normal
 * update inside two or three taps.
 */
@Composable
fun JobDetailsScreen(
    onBack: () -> Unit,
    onTakePhoto: (PhotoKind) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: JobDetailsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val job by viewModel.job.collectAsStateWithLifecycle()
    val technicians by viewModel.technicians.collectAsStateWithLifecycle()
    val partsResults by viewModel.partsResults.collectAsStateWithLifecycle()
    val context = LocalContext.current
    var menuOpen by remember { mutableStateOf(false) }

    val detail = job

    TlScreen(
        title = detail?.jobCode ?: "Job",
        subtitle = detail?.let { "${it.customer.name ?: "Walk-in"} · ${it.device.label}" },
        onBack = onBack,
        modifier = modifier,
        onRefresh = { viewModel.refresh() },
        refreshing = state.refreshing,
        actions = {
            IconButton(onClick = { menuOpen = true }) {
                Icon(Icons.Outlined.MoreVert, contentDescription = "More")
            }
            DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                DropdownMenuItem(
                    text = { Text("Update status") },
                    onClick = { menuOpen = false; viewModel.openSheet(JobSheet.Status) },
                )
                DropdownMenuItem(
                    text = { Text(if (detail?.technicianId == null) "Assign technician" else "Reassign technician") },
                    onClick = { menuOpen = false; viewModel.openSheet(JobSheet.Technician) },
                )
                DropdownMenuItem(
                    text = { Text("Send customer update") },
                    onClick = { menuOpen = false; viewModel.openSheet(JobSheet.CustomerUpdate) },
                )
                DropdownMenuItem(
                    text = { Text("Reprint intake slip") },
                    leadingIcon = { Icon(Icons.Outlined.Print, contentDescription = null) },
                    enabled = !state.printingReceipt,
                    onClick = { menuOpen = false; viewModel.reprintIntakeSlip() },
                )
                DropdownMenuItem(
                    text = { Text("Reprint final receipt") },
                    leadingIcon = { Icon(Icons.Outlined.Print, contentDescription = null) },
                    enabled = !state.printingReceipt,
                    onClick = { menuOpen = false; viewModel.reprintFinalReceipt() },
                )
                DropdownMenuItem(
                    text = { Text("Refresh") },
                    leadingIcon = { Icon(Icons.Outlined.Refresh, contentDescription = null) },
                    onClick = { menuOpen = false; viewModel.refresh() },
                )
            }
        },
        footer = detail?.let {
            {
                QuickActions(
                    detail = it,
                    onAction = { action -> viewModel.handle(action, onTakePhoto) },
                )
            }
        },
    ) {
        if (detail == null) {
            if (state.loading) TlLoading(label = "Loading job…") else {
                TlBanner(
                    message = state.error ?: "This job isn't cached on this phone yet. Pull down to fetch it.",
                    tone = TlTone.Warning,
                )
            }
            return@TlScreen
        }

        TlBanner(message = state.error, tone = TlTone.Warning)
        TlBanner(message = state.message, tone = TlTone.Success)
        SyncStatusIndicator(pendingCount = detail.pendingSyncCount)

        StatusHeader(detail = detail, onChangeStatus = { viewModel.openSheet(JobSheet.Status) })

        CustomerDeviceCard(
            detail = detail,
            onCall = { phone ->
                runCatching {
                    context.startActivity(Intent(Intent.ACTION_DIAL, Uri.parse("tel:$phone")))
                }
            },
            technicianName = technicians.firstOrNull { it.id == detail.technicianId }?.displayName,
            onAssign = { viewModel.openSheet(JobSheet.Technician) },
        )

        TlCard {
            Text("Reported problem", style = MaterialTheme.typography.titleSmall)
            Text(
                detail.problemSummary.ifBlank { "No complaint recorded at intake." },
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
            )
        }

        DiagnosisCard(detail = detail, onEdit = { viewModel.openSheet(JobSheet.Diagnosis) })

        ApprovalCard(
            approval = detail.approval,
            pendingEstimateTotal = detail.pendingEstimate?.total,
            onSendEstimate = { viewModel.openSheet(JobSheet.Estimate) },
            onRecordApproval = { viewModel.openSheet(JobSheet.Approval) },
        )

        PartsCard(
            parts = detail.parts,
            canEdit = detail.status.isOpen,
            onAddPart = { viewModel.openSheet(JobSheet.Parts) },
            onRemovePart = viewModel::removePart,
            onMarkPartRequired = viewModel::markPartRequired,
        )

        RepairPhotoGallery(
            photos = detail.photos,
            onAddPhoto = { viewModel.openSheet(JobSheet.Photo) },
            onDeletePhoto = viewModel::deletePhoto,
            photoUrl = viewModel::photoUrl,
        )

        RepairTimeline(events = detail.timeline())

        Column(Modifier.height(TlTheme.spacing.sm)) {}
    }

    // ---------------------------------------------------------------- sheets

    when (state.sheet) {
        JobSheet.None -> Unit

        JobSheet.Status -> detail?.let {
            StatusPickerBottomSheet(
                current = it.status,
                blockedReason = if (it.needsApprovalBeforeBench) {
                    "Customer approval is required before repair work can begin."
                } else {
                    null
                },
                onSelect = viewModel::changeStatus,
                onRequestApproval = { viewModel.openSheet(JobSheet.Approval) },
                onDismiss = viewModel::closeSheet,
            )
        }

        JobSheet.Technician -> TechnicianPicker(
            technicians = technicians,
            currentId = detail?.technicianId,
            meId = state.meId,
            onSelect = { viewModel.assign(it.id) },
            onAssignToMe = viewModel::assignToMe,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.Diagnosis -> DiagnosisSheet(
            existing = null,
            onSave = viewModel::addDiagnosis,
            onMarkInconclusive = viewModel::markDiagnosisInconclusive,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.Estimate -> EstimateSheet(
            suggested = detail?.let { it.laborAmount + it.parts.sumOf { p -> p.lineTotal } },
            onSend = viewModel::sendEstimate,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.Approval -> RecordApprovalSheet(
            suggestedAmount = detail?.pendingEstimate?.total ?: detail?.amountDue,
            onRecord = viewModel::recordVerbalApproval,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.Parts -> PartsPickerSheet(
            results = partsResults,
            query = state.partsQuery,
            onQueryChange = viewModel::setPartsQuery,
            onAdd = viewModel::addPart,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.CustomerUpdate -> CustomerUpdateSheet(
            templates = viewModel.templatesForCurrentStatus(),
            context = viewModel.updateContext(),
            customerName = detail?.customer?.name,
            hasPhone = !detail?.customer?.phone.isNullOrBlank(),
            onSend = viewModel::sendCustomerUpdate,
            onDismiss = viewModel::closeSheet,
        )

        JobSheet.Photo -> PhotoKindSheet(
            onPick = { kind ->
                viewModel.setPendingPhotoKind(kind)
                viewModel.closeSheet()
                onTakePhoto(kind)
            },
            onDismiss = viewModel::closeSheet,
        )
    }
}

@Composable
private fun StatusHeader(detail: JobDetail, onChangeStatus: () -> Unit) {
    TlCard(onClick = onChangeStatus) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column {
                Text(
                    "Current status",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(detail.status.label, style = MaterialTheme.typography.titleLarge)
            }
            JobStatusChip(detail.status)
        }
        if (detail.stale) {
            Text(
                "Last synced a while ago — pull down to refresh.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun CustomerDeviceCard(
    detail: JobDetail,
    technicianName: String?,
    onCall: (String) -> Unit,
    onAssign: () -> Unit,
) {
    TlCard {
        Text("Customer & device", style = MaterialTheme.typography.titleSmall)
        TlKeyValue("Customer", detail.customer.name ?: "Walk-in")
        detail.customer.phone?.let { phone ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "Phone",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Surface(
                    onClick = { onCall(phone) },
                    shape = MaterialTheme.shapes.extraSmall,
                    color = MaterialTheme.colorScheme.surface,
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Icon(
                            Icons.Outlined.Call,
                            contentDescription = "Call $phone",
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.height(16.dp),
                        )
                        Text(
                            Msisdn.formatLocal(phone),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary,
                        )
                    }
                }
            }
        }
        TlKeyValue("Device", detail.device.label)
        TlKeyValue("Type", detail.device.kind.replaceFirstChar(Char::uppercase))
        detail.device.identifier?.let { (label, value) -> TlKeyValue(label, value) }
        TlKeyValue("Taken in", dateFormat.format(Date(detail.createdAt)))
        detail.promisedBy?.let { TlKeyValue("Promised", dateFormat.format(Date(it))) }
        if (detail.amountDue > 0) {
            TlKeyValue("Amount due", formatKes(detail.balanceDue), emphasise = true)
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Technician",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            TlSecondaryButton(
                text = technicianName ?: "Assign",
                onClick = onAssign,
            )
        }
    }
}

@Composable
private fun DiagnosisCard(detail: JobDetail, onEdit: () -> Unit) {
    TlCard {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("Diagnosis", style = MaterialTheme.typography.titleSmall)
            if (detail.notes.isNotEmpty()) {
                Text(
                    "${detail.notes.size} ${if (detail.notes.size == 1) "entry" else "entries"}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
        val latest = detail.latestDiagnosis
        if (latest == null) {
            Text(
                "No diagnosis recorded yet.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        } else {
            Text(latest.text, style = MaterialTheme.typography.bodyMedium)
            Text(
                buildString {
                    append(dateFormat.format(Date(latest.createdAt)))
                    latest.authorName?.let { append(" · ").append(it) }
                    if (latest.pendingSync) append(" · not synced")
                },
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        TlSecondaryButton(
            text = if (latest == null) "Add diagnosis" else "Update diagnosis",
            onClick = onEdit,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/**
 * Contextual actions only. The full set lives behind the overflow menu — showing
 * everything at once turns the footer into a menu the technician has to read.
 */
@Composable
private fun QuickActions(detail: JobDetail, onAction: (JobAction) -> Unit) {
    val actions = JobAction.forStatus(detail.status, detail.needsApprovalBeforeBench)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        actions.forEachIndexed { index, action ->
            if (index == 0) {
                TlButton(text = action.label, onClick = { onAction(action) })
            } else {
                TlSecondaryButton(text = action.label, onClick = { onAction(action) })
            }
        }
    }
}

/** Maps a quick action onto the sheet or mutation that performs it. */
private fun JobDetailsViewModel.handle(action: JobAction, onTakePhoto: (PhotoKind) -> Unit) {
    when (action) {
        JobAction.AddDiagnosis, JobAction.AddNote -> openSheet(JobSheet.Diagnosis)
        JobAction.AddPhoto -> openSheet(JobSheet.Photo)
        JobAction.SendEstimate -> openSheet(JobSheet.Estimate)
        JobAction.RecordApproval -> openSheet(JobSheet.Approval)
        JobAction.AddPart -> openSheet(JobSheet.Parts)
        JobAction.NotifyCustomer -> openSheet(JobSheet.CustomerUpdate)
        JobAction.UpdateStatus -> openSheet(JobSheet.Status)
        JobAction.ResumeRepair -> changeStatus(JobStatus.InProgress, "Resuming — part received", null)
        JobAction.MarkReady -> openSheet(JobSheet.Status)
        JobAction.MarkComplete -> openSheet(JobSheet.Status)
    }
}

private val dateFormat = SimpleDateFormat("d MMM yyyy, h:mm a", Locale.getDefault())
