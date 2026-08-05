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
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
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
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTextButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.JobAction
import com.techlane.pos.domain.model.JobDetail
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.StkStage
import com.techlane.pos.feature.charge.StkStatusScreen
import com.techlane.pos.feature.jobs.components.AddServiceSheet
import com.techlane.pos.feature.jobs.components.ApprovalCard
import com.techlane.pos.feature.jobs.components.CollectHandoverSheet
import com.techlane.pos.feature.jobs.components.CustomerUpdateSheet
import com.techlane.pos.feature.jobs.components.DiagnosisSheet
import com.techlane.pos.feature.jobs.components.EstimateSheet
import com.techlane.pos.feature.jobs.components.JobStatusChip
import com.techlane.pos.feature.jobs.components.LineItemsCard
import com.techlane.pos.feature.jobs.components.PartsPickerSheet
import com.techlane.pos.feature.jobs.components.PhotoKindSheet
import com.techlane.pos.feature.jobs.components.RecordApprovalSheet
import com.techlane.pos.feature.jobs.components.RepairPhotoGallery
import com.techlane.pos.feature.jobs.components.RepairTimeline
import com.techlane.pos.feature.jobs.components.StatusPickerBottomSheet
import com.techlane.pos.feature.jobs.components.SyncStatusIndicator
import com.techlane.pos.feature.jobs.components.TakePaymentSheet
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

        MoneyCard(
            total = detail.amountDue,
            paid = detail.paidTotal,
            balance = detail.balanceDue,
            methodSummary = paymentMethodSummary(state.payments),
            payments = state.payments,
            unmatchedC2b = state.unmatchedC2b,
            busy = state.paymentStage != null || state.busy,
            onTakePayment = viewModel::openTakePayment,
            onMatchC2b = viewModel::matchC2b,
            onPrintReceipt = viewModel::reprintFinalReceipt,
        )

        LineItemsCard(
            title = "Labour",
            parts = detail.parts.filter { it.lineType == "labour" },
            canEdit = detail.status.isOpen,
            onAdd = { viewModel.openSheet(JobSheet.AddService) },
            addLabel = "Add service",
            emptyLabel = "No labour lines yet.",
            onRemove = viewModel::removePart,
        )

        LineItemsCard(
            title = "Parts",
            parts = detail.parts.filter { it.lineType == "part" },
            canEdit = detail.status.isOpen,
            onAdd = { viewModel.openSheet(JobSheet.Parts) },
            addLabel = "Add part",
            emptyLabel = "No parts on this job yet.",
            onRemove = viewModel::removePart,
            onMarkPartRequired = viewModel::markPartRequired,
        )

        LineItemsCard(
            title = "Products",
            parts = detail.parts.filter { it.lineType == "product" },
            canEdit = detail.status.isOpen,
            onAdd = { viewModel.openSheet(JobSheet.AddProduct) },
            addLabel = "Add product",
            emptyLabel = "No products added yet.",
            onRemove = viewModel::removePart,
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
                balanceDue = it.balanceDue,
                onSelect = viewModel::changeStatus,
                onRequestApproval = { viewModel.openSheet(JobSheet.Approval) },
                onTakePayment = viewModel::openTakePayment,
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
            title = "Add part",
            onAddSourced = viewModel::addSourcedPart,
        )

        JobSheet.AddProduct -> PartsPickerSheet(
            results = partsResults,
            query = state.partsQuery,
            onQueryChange = viewModel::setPartsQuery,
            onAdd = viewModel::addProduct,
            onDismiss = viewModel::closeSheet,
            title = "Add product",
        )

        JobSheet.AddService -> AddServiceSheet(
            onAdd = viewModel::addLabourLine,
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

        JobSheet.TakePayment -> detail?.let {
            TakePaymentSheet(
                balanceDue = it.balanceDue,
                customerPhone = it.customer.phone,
                jobCode = it.jobCode,
                onConfirm = viewModel::takePayment,
                onDismiss = viewModel::closeSheet,
            )
        }

        JobSheet.Collect -> detail?.let {
            CollectHandoverSheet(
                customerName = it.customer.name,
                balanceDue = it.balanceDue,
                canVouch = state.canReleaseUnverified,
                sendingCode = state.sendingHandoverCode,
                onConfirm = viewModel::recordHandover,
                onSendCode = viewModel::sendHandoverCode,
                onTakePayment = viewModel::openTakePayment,
                onDismiss = viewModel::closeSheet,
            )
        }
    }

    // The payment result covers this screen rather than navigating away, so
    // "Done" simply closes it and the operator is already back on the job —
    // no back-stack unwinding, and no way to land on a stale payment screen.
    state.paymentStage?.let { stage ->
        val balanceAfter = detail?.balanceDue ?: 0.0
        StkStatusScreen(
            stage = stage,
            amount = state.paymentAmount.takeIf { it > 0 } ?: (detail?.balanceDue ?: 0.0),
            phone = state.paymentPhone ?: detail?.customer?.phone,
            label = detail?.let { "${it.jobCode} · ${it.device.label}" }.orEmpty(),
            method = state.paymentMethod,
            canForceReconcile = state.canForceReconcile,
            receiptBusy = state.printingReceipt,
            receiptError = null,
            onPrintReceipt = viewModel::reprintFinalReceipt,
            onShareReceipt = viewModel::reprintFinalReceipt,
            onRetry = viewModel::retryPayment,
            onTakeCash = viewModel::takeCashInstead,
            onCheckAgain = viewModel::retryPayment,
            onStopWaiting = viewModel::dismissPayment,
            onDone = viewModel::dismissPayment,
            onDismiss = viewModel::dismissPayment,
            onViewJob = null,
            onMarkCollected = if (stage is StkStage.Paid &&
                state.offerCollectAfterPay
            ) {
                {
                    viewModel.dismissPayment()
                    viewModel.openCollect()
                }
            } else {
                null
            },
            onTakeAnotherPayment = if (stage is StkStage.Paid &&
                !state.offerCollectAfterPay && balanceAfter > 0.009
            ) {
                {
                    viewModel.dismissPayment()
                    viewModel.openTakePayment()
                }
            } else {
                null
            },
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

/**
 * What the customer owes, and the one action that settles it. Hidden entirely
 * on a job with no money attached — an empty totals block on a warranty repair
 * is noise the counter has to read past.
 */
@Composable
private fun MoneyCard(
    total: Double,
    paid: Double,
    balance: Double,
    methodSummary: String?,
    payments: List<com.techlane.pos.data.remote.dto.PaymentDto>,
    unmatchedC2b: List<com.techlane.pos.data.remote.dto.C2bTransactionDto>,
    busy: Boolean,
    onTakePayment: () -> Unit,
    onMatchC2b: (String) -> Unit,
    onPrintReceipt: () -> Unit,
) {
    if (total <= 0.0 && balance <= 0.0 && payments.isEmpty()) return
    val settled = balance <= 0.009
    TlCard {
        Text("PAYMENT", style = MaterialTheme.typography.titleSmall)
        TlKeyValue("Total", formatKes(total))
        TlKeyValue("Paid", formatKes(paid))
        TlKeyValue(
            label = if (settled) "Balance" else "Balance due",
            value = formatKes(balance),
            emphasise = true,
            valueColor = if (settled) TlTheme.colors.success else MaterialTheme.colorScheme.error,
        )
        if (!methodSummary.isNullOrBlank()) {
            TlKeyValue("Methods", methodSummary)
        }
        if (settled) {
            TlStatusPill(text = "PAID IN FULL", tone = TlTone.Success, leadingDot = false)
        } else {
            TlButton(
                text = "Take Payment · ${formatKes(balance)}",
                onClick = onTakePayment,
                enabled = !busy,
                modifier = Modifier.fillMaxWidth(),
                large = true,
            )
        }

        if (unmatchedC2b.isNotEmpty() && !settled) {
            Text("Recent unmatched payments", style = MaterialTheme.typography.titleSmall)
            unmatchedC2b.take(5).forEach { row ->
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                ) {
                    Text(
                        listOfNotNull(
                            row.transId.takeIf { it.isNotBlank() }?.let { "MPESA $it" },
                            row.msisdn.takeIf { it.isNotBlank() },
                            formatKes(row.amount),
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    TlSecondaryButton(
                        text = "Match to Job",
                        onClick = { onMatchC2b(row.id) },
                        enabled = !busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }

        val history = payments.filter { it.isSettled || it.status == "initiated" }
        if (history.isNotEmpty()) {
            Text("Payment History", style = MaterialTheme.typography.titleSmall)
            history.take(8).forEach { pay ->
                val methodLabel = when (pay.method) {
                    "cash" -> "Cash"
                    "mpesa_stk" -> "M-Pesa"
                    "mpesa_c2b" -> "Paybill"
                    else -> pay.method
                }
                val statusLabel = when {
                    pay.isSettled -> "Confirmed"
                    pay.isFailed -> "Failed"
                    else -> "Pending"
                }
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(2.dp),
                ) {
                    Text(
                        listOfNotNull(
                            pay.createdAt?.let { formatPaymentTime(it) },
                            methodLabel,
                            formatKes(pay.amount),
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Text(
                        listOfNotNull(
                            pay.displayReference.takeIf { it.isNotBlank() },
                            statusLabel,
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    if (pay.isSettled) {
                        TlTextButton(
                            text = "View / print receipt",
                            onClick = onPrintReceipt,
                        )
                    }
                }
            }
        }
    }
}

private fun paymentMethodSummary(payments: List<com.techlane.pos.data.remote.dto.PaymentDto>): String? {
    val labels = payments.filter { it.isSettled }.mapNotNull {
        when (it.method) {
            "cash" -> "Cash"
            "mpesa_stk", "mpesa_c2b" -> "M-Pesa"
            else -> null
        }
    }.distinct()
    return when {
        labels.isEmpty() -> null
        labels.size == 1 -> labels.first()
        else -> labels.joinToString(" + ")
    }
}

private fun formatPaymentTime(iso: String): String {
    return runCatching {
        val instant = java.time.Instant.parse(iso)
        dateFormat.format(Date.from(instant))
    }.getOrElse { iso }
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
    val actions = JobAction.forStatus(detail.status, detail.needsApprovalBeforeBench, detail.balanceDue)
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
        JobAction.MarkCollected -> openCollect()
        JobAction.TakePayment -> openTakePayment()
    }
}

private val dateFormat = SimpleDateFormat("d MMM yyyy, h:mm a", Locale.getDefault())
