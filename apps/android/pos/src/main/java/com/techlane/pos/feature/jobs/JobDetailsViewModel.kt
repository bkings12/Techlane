package com.techlane.pos.feature.jobs

import android.content.Context
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.core.print.ReceiptPrinter
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.data.local.TechnicianEntity
import com.techlane.pos.data.printer.PrinterRepository
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.IssueWifiVoucherRequest
import com.techlane.pos.data.remote.toAppException
import com.techlane.pos.data.repository.ChargeRepository
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.repository.ShopRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.ClosureReason
import com.techlane.pos.domain.model.CustomerUpdateContext
import com.techlane.pos.domain.model.CustomerUpdateTemplate
import com.techlane.pos.domain.model.CustomerUpdateTemplates
import com.techlane.pos.domain.model.JobDetail
import com.techlane.pos.domain.model.JobPart
import com.techlane.pos.domain.model.JobPhoto
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.MpesaReference
import com.techlane.pos.domain.model.PaymentMethod
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.StkStage
import com.techlane.pos.domain.model.VerbalApprovalChannel
import com.techlane.pos.sync.JobSyncWorker
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.File
import java.util.UUID
import javax.inject.Inject

/** Which secondary surface, if any, is open over the detail screen. */
enum class JobSheet { None, Status, Technician, Diagnosis, Estimate, Approval, Parts, AddService, AddProduct, CustomerUpdate, Photo, TakePayment, Collect }

data class JobDetailsUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val error: String? = null,
    val message: String? = null,
    val sheet: JobSheet = JobSheet.None,
    val meId: String? = null,
    val locationId: String? = null,
    val busy: Boolean = false,
    val partsQuery: String = "",
    val pendingPhotoKind: PhotoKind = PhotoKind.Progress,
    val printingReceipt: Boolean = false,
    val issuingWifi: Boolean = false,
    val wifiSlipHtml: String? = null,
    /** Non-null while a job payment is in flight or resolved — drives the STK sheet. */
    val paymentStage: StkStage? = null,
    val paymentMethod: PaymentMethod = PaymentMethod.MpesaStk,
    /** Amount of the payment currently in flight (may be partial). */
    val paymentAmount: Double = 0.0,
    val paymentPhone: String? = null,
    val branchId: String? = null,
    val canForceReconcile: Boolean = false,
    val canReleaseUnverified: Boolean = false,
    val payments: List<com.techlane.pos.data.remote.dto.PaymentDto> = emptyList(),
    val unmatchedC2b: List<com.techlane.pos.data.remote.dto.C2bTransactionDto> = emptyList(),
    val sendingHandoverCode: Boolean = false,
    /** After a confirmed payment with zero balance, nudge collection. */
    val offerCollectAfterPay: Boolean = false,
)

@OptIn(ExperimentalCoroutinesApi::class)
@HiltViewModel
class JobDetailsViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val jobs: JobRepository,
    private val shop: ShopRepository,
    private val prefs: PreferencesStore,
    private val printers: PrinterRepository,
    private val charges: ChargeRepository,
    private val api: TechLaneApi,
    @ApplicationContext private val context: Context,
) : ViewModel() {

    val jobId: String = checkNotNull(savedStateHandle["jobId"]) { "jobId is required" }

    private var paymentJob: Job? = null

    private val _state = MutableStateFlow(JobDetailsUiState())
    val state: StateFlow<JobDetailsUiState> = _state.asStateFlow()

    val job: StateFlow<JobDetail?> = jobs.observeJob(jobId)
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), null)

    val technicians: StateFlow<List<TechnicianEntity>> = jobs.observeTechnicians()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    val partsResults: StateFlow<List<CatalogItemEntity>> = _state
        .map { it.locationId to it.partsQuery }
        .flatMapLatest { (locationId, query) ->
            when {
                locationId.isNullOrBlank() -> flowOf(emptyList())
                query.isBlank() -> shop.observeCatalog(locationId)
                else -> shop.searchCatalog(locationId, query)
            }
        }
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    init {
        viewModelScope.launch {
            val preferences = prefs.preferences.first()
            _state.update {
                it.copy(
                    meId = preferences.userId,
                    locationId = preferences.locationId,
                    branchId = preferences.branchId,
                    canForceReconcile = preferences.canForceReconcile,
                    canReleaseUnverified = preferences.canReleaseUnverified,
                )
            }
        }
        refresh(initial = true)
    }

    // ------------------------------------------------------------- payment
    //
    // Settles the job's balance against the job itself rather than creating a
    // sale — see ChargeRepository.chargeRepair. The job is refreshed on success
    // so the balance the screen shows is the server's, never a local guess.

    fun openTakePayment() = _state.update {
        it.copy(sheet = JobSheet.TakePayment, message = null, error = null)
    }

    fun openCollect() = _state.update {
        it.copy(sheet = JobSheet.Collect, message = null, error = null, offerCollectAfterPay = false)
    }

    fun takePayment(draft: com.techlane.pos.feature.jobs.components.TakePaymentDraft) {
        val detail = job.value ?: return
        val branchId = _state.value.branchId
        if (branchId == null) {
            _state.update { it.copy(error = "Pick a branch in Settings before taking payment.") }
            return
        }
        if (detail.balanceDue <= 0.0) return
        if (_state.value.paymentStage != null) return
        if (draft.amount <= 0.0 || draft.amount > detail.balanceDue + 0.009) {
            _state.update { it.copy(error = "Enter an amount up to the outstanding balance.") }
            return
        }
        if (draft.method == PaymentMethod.Paybill && !draft.reference.isNullOrBlank()) {
            MpesaReference.validationError(draft.reference)?.let { problem ->
                _state.update { it.copy(error = problem) }
                return
            }
        }

        _state.update {
            it.copy(
                paymentMethod = draft.method,
                paymentAmount = draft.amount,
                paymentPhone = draft.phone ?: detail.customer.phone,
                sheet = JobSheet.None,
                error = null,
                message = null,
                offerCollectAfterPay = false,
            )
        }
        paymentJob?.cancel()
        paymentJob = viewModelScope.launch {
            charges.chargeRepair(
                repairId = jobId,
                branchId = branchId,
                amount = draft.amount,
                method = draft.method,
                phone = draft.phone ?: detail.customer.phone,
                label = "${detail.jobCode} · ${detail.device.label}",
                idempotencyKey = UUID.randomUUID().toString(),
                reference = draft.reference?.let(MpesaReference::normalise),
                jobCode = detail.jobCode,
            ).collect { stage ->
                _state.update { it.copy(paymentStage = stage) }
                if (stage is StkStage.Paid) {
                    jobs.refreshJob(jobId)
                    loadPayments()
                    val remaining = (detail.balanceDue - stage.amount).coerceAtLeast(0.0)
                    _state.update {
                        it.copy(offerCollectAfterPay = remaining <= 0.009)
                    }
                }
            }
        }
    }

    /** Legacy entry used by retry / take-cash-instead after a failed prompt. */
    fun takePayment(method: PaymentMethod, reference: String? = null) {
        val detail = job.value ?: return
        takePayment(
            com.techlane.pos.feature.jobs.components.TakePaymentDraft(
                method = method,
                amount = detail.balanceDue,
                phone = detail.customer.phone,
                reference = reference,
            ),
        )
    }

    /** Closes the payment sheet. Never cancels a prompt that is still in flight. */
    fun dismissPayment() {
        val stage = _state.value.paymentStage
        if (stage is StkStage.Sending || stage is StkStage.Waiting || stage is StkStage.Finalising) return
        paymentJob?.cancel()
        paymentJob = null
        val offerCollect = _state.value.offerCollectAfterPay
        _state.update { it.copy(paymentStage = null) }
        if (offerCollect) openCollect()
    }

    fun retryPayment() {
        _state.update { it.copy(paymentStage = null) }
        takePayment(_state.value.paymentMethod)
    }

    fun takeCashInstead() {
        _state.update { it.copy(paymentStage = null) }
        takePayment(PaymentMethod.Cash)
    }

    fun matchC2b(c2bId: String) {
        val detail = job.value ?: return
        val branchId = _state.value.branchId ?: return
        if (_state.value.busy) return
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            runCatching {
                val payment = charges.createRepairPaymentPending(
                    repairId = jobId,
                    branchId = branchId,
                    amount = detail.balanceDue,
                    jobCode = detail.jobCode,
                    phone = detail.customer.phone,
                )
                jobs.matchC2bToPayment(c2bId, payment.id).getOrThrow()
                jobs.refreshJob(jobId)
                loadPayments()
                loadUnmatchedC2b()
                _state.update {
                    it.copy(
                        message = "Paybill matched",
                        offerCollectAfterPay = (job.value?.balanceDue ?: 1.0) <= 0.009,
                    )
                }
            }.onFailure { e ->
                _state.update { it.copy(error = e.message ?: "Could not match Paybill") }
            }
            _state.update { it.copy(busy = false) }
        }
    }

    fun recordHandover(
        collectedByName: String,
        relationship: String,
        note: String?,
        pickupCode: String?,
        otp: String?,
    ) {
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            jobs.recordHandover(jobId, collectedByName, relationship, note, pickupCode, otp)
                .onSuccess {
                    _state.update {
                        it.copy(
                            sheet = JobSheet.None,
                            message = "Device collected",
                            offerCollectAfterPay = false,
                        )
                    }
                }
                .onFailure { e ->
                    _state.update { it.copy(error = e.message ?: "Could not record handover") }
                }
            _state.update { it.copy(busy = false) }
        }
    }

    fun sendHandoverCode() {
        if (_state.value.sendingHandoverCode) return
        _state.update { it.copy(sendingHandoverCode = true, error = null) }
        viewModelScope.launch {
            jobs.sendHandoverCode(jobId)
                .onSuccess { _state.update { it.copy(message = "Handover code sent") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not send code") } }
            _state.update { it.copy(sendingHandoverCode = false) }
        }
    }

    private suspend fun loadPayments() {
        jobs.listRepairPayments(jobId)
            .onSuccess { list -> _state.update { it.copy(payments = list) } }
    }

    private suspend fun loadUnmatchedC2b() {
        jobs.listUnmatchedC2b()
            .onSuccess { list -> _state.update { it.copy(unmatchedC2b = list) } }
    }

    fun refresh(initial: Boolean = false) {
        _state.update { it.copy(refreshing = !initial, loading = initial && job.value == null, error = null) }
        viewModelScope.launch {
            jobs.refreshJob(jobId).onFailure { error ->
                // Cached detail is still the technician's working copy.
                _state.update { it.copy(error = error.message) }
            }
            loadPayments()
            loadUnmatchedC2b()
            _state.update { it.copy(refreshing = false, loading = false) }
        }
    }

    fun openSheet(sheet: JobSheet) = _state.update { it.copy(sheet = sheet, message = null) }
    fun closeSheet() = _state.update { it.copy(sheet = JobSheet.None) }
    fun clearFeedback() = _state.update { it.copy(message = null, error = null) }
    fun setPartsQuery(value: String) = _state.update { it.copy(partsQuery = value) }
    fun setPendingPhotoKind(kind: PhotoKind) = _state.update { it.copy(pendingPhotoKind = kind) }

    /** Reprints the slip handed over at intake — the customer's copy, not a shop record. */
    fun reprintIntakeSlip() = reprint("intake slip") { printers.printIntakeSlip(jobId) }

    /** Reprints the job's final receipt — what the customer gets on collection. */
    fun reprintFinalReceipt() = reprint("receipt") { printers.printRepairReceipt(jobId) }

    fun dismissWifiSlip() = _state.update { it.copy(wifiSlipHtml = null) }

    /**
     * Issues a complimentary BytePesa Guest WiFi voucher for the waiting customer
     * and opens the printable QR slip (system print sheet).
     */
    fun giveGuestWifi() {
        if (_state.value.issuingWifi) return
        val detail = job.value
        _state.update { it.copy(issuingWifi = true, error = null, message = null, wifiSlipHtml = null) }
        viewModelScope.launch {
            try {
                val voucher = api.issueWifiVoucher(
                    IssueWifiVoucherRequest(
                        phone = detail?.customer?.phone,
                        repairId = jobId,
                        reference = detail?.jobCode ?: jobId,
                    ),
                )
                val html = api.wifiVoucherSlipHtml(voucher.id).use { it.string() }
                ReceiptPrinter.print(context, html, "Guest WiFi ${voucher.code}")
                _state.update {
                    it.copy(
                        message = "Guest WiFi ${voucher.code} · ${voucher.durationMins} min",
                        wifiSlipHtml = html,
                    )
                }
            } catch (e: Exception) {
                _state.update {
                    it.copy(error = e.toAppException().message ?: "Could not issue Guest WiFi")
                }
            } finally {
                _state.update { it.copy(issuingWifi = false) }
            }
        }
    }

    private fun reprint(label: String, action: suspend () -> Result<Unit>) {
        if (_state.value.printingReceipt) return
        _state.update { it.copy(printingReceipt = true, error = null, message = null) }
        viewModelScope.launch {
            action()
                .onSuccess { _state.update { it.copy(message = "Sent the $label to the printer") } }
                .onFailure { error -> _state.update { it.copy(error = error.message ?: "Could not print the $label") } }
            _state.update { it.copy(printingReceipt = false) }
        }
    }

    // ------------------------------------------------------------- mutations
    //
    // Every one of these writes to Room first and queues the network call, so a
    // technician in a back room sees their own change immediately. `kick()` only
    // asks WorkManager to try; it never blocks the UI on the result.

    fun changeStatus(status: JobStatus, note: String?, closureReason: ClosureReason?) {
        if (status == JobStatus.Collected) {
            openCollect()
            return
        }
        viewModelScope.launch {
            jobs.changeStatus(jobId, status, note, closureReason?.wire)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Moved to ${status.label}") }
        }
    }

    fun assign(technicianId: String) {
        viewModelScope.launch {
            jobs.assign(jobId, technicianId)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Technician assigned") }
        }
    }

    fun assignToMe() {
        val me = _state.value.meId ?: return
        assign(me)
    }

    fun addDiagnosis(text: String) {
        if (text.isBlank()) return
        viewModelScope.launch {
            val name = prefs.preferences.first().displayName
            jobs.addNote(jobId, text.trim(), name)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Diagnosis saved") }
        }
    }

    /**
     * "Inconclusive" is a note, not a status: the backend has no such state, and
     * inventing one locally would make this handset disagree with the console.
     */
    fun markDiagnosisInconclusive() {
        addDiagnosis("Diagnosis inconclusive — further testing required.")
    }

    fun sendEstimate(total: Double, notes: String?) {
        viewModelScope.launch {
            jobs.sendEstimate(jobId, total, notes)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Estimate queued for the customer") }
        }
    }

    /**
     * Records a go-ahead obtained away from the estimate flow. The channel and
     * who took it go into the authorization note, which is the audit trail the
     * server keeps — this is never a silent unlock.
     */
    fun recordVerbalApproval(channel: VerbalApprovalChannel, amount: Double?, detail: String?) {
        viewModelScope.launch {
            val who = prefs.preferences.first().displayName ?: "staff"
            val note = buildString {
                append(channel.label)
                append(" approval recorded by ").append(who)
                if (!detail.isNullOrBlank()) append(" — ").append(detail.trim())
            }
            jobs.authorizeWork(jobId, note, amount)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Approval recorded") }
        }
    }

    // Labour, parts, and products are all online-only (see JobRepository) — the
    // server resolves catalog price/cost and deducts stock, none of which a
    // phone can safely invent offline. These calls surface busy/error state
    // rather than firing-and-forgetting into the outbox the way most mutations
    // above do.

    fun addPart(item: CatalogItemEntity, quantity: Int) {
        val locationId = _state.value.locationId
        if (locationId.isNullOrBlank()) {
            _state.update { it.copy(error = "Pick a stock location in Settings before adding parts.") }
            return
        }
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            jobs.addInventoryPartLine(jobId, item.variantId, locationId, quantity.toDouble())
                .onSuccess { _state.update { it.copy(sheet = JobSheet.None, message = "${item.productName} added") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not add the part") } }
            _state.update { it.copy(busy = false) }
        }
    }

    fun addSourcedPart(
        description: String,
        unitCost: Double,
        unitPrice: Double,
        quantity: Int,
        supplierName: String?,
    ) {
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            jobs.addSourcedPartLine(jobId, description, unitCost, unitPrice, quantity.toDouble(), supplierName)
                .onSuccess { _state.update { it.copy(sheet = JobSheet.None, message = "$description added") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not add the part") } }
            _state.update { it.copy(busy = false) }
        }
    }

    fun addLabourLine(description: String, unitPrice: Double, quantity: Int = 1) {
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            jobs.addLabourLine(jobId, description, unitPrice, quantity.toDouble())
                .onSuccess { _state.update { it.copy(sheet = JobSheet.None, message = "$description added") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not add the service") } }
            _state.update { it.copy(busy = false) }
        }
    }

    fun addProduct(item: CatalogItemEntity, quantity: Int) {
        val locationId = _state.value.locationId
        if (locationId.isNullOrBlank()) {
            _state.update { it.copy(error = "Pick a stock location in Settings before adding products.") }
            return
        }
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            jobs.addProductLine(jobId, item.variantId, locationId, quantity.toDouble())
                .onSuccess { _state.update { it.copy(sheet = JobSheet.None, message = "${item.productName} added") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not add the product") } }
            _state.update { it.copy(busy = false) }
        }
    }

    fun removePart(part: JobPart) {
        viewModelScope.launch {
            jobs.removeLineItem(jobId, part)
                .onSuccess { _state.update { it.copy(message = "${part.name} removed") } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not remove ${part.name}") } }
        }
    }

    /** "Part required" with no stock: park the job rather than leave it on the bench. */
    fun markPartRequired() {
        viewModelScope.launch {
            jobs.addNote(jobId, "Part required — not in stock.", prefs.preferences.first().displayName)
            val current = job.value?.status
            if (current != null && JobStatus.WaitingParts in current.allowedNext) {
                jobs.changeStatus(jobId, JobStatus.WaitingParts, "Waiting for a part", null)
            }
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Marked as waiting for parts") }
        }
    }

    fun addPhoto(file: File, kind: PhotoKind, caption: String?) {
        viewModelScope.launch {
            jobs.addPhoto(jobId, file, kind, caption?.takeIf { it.isNotBlank() })
            kick()
            _state.update { it.copy(message = "Photo added") }
        }
    }

    fun deletePhoto(photo: JobPhoto) {
        if (!photo.canDelete) {
            _state.update { it.copy(error = "This photo is already on the server and can't be deleted here.") }
            return
        }
        viewModelScope.launch { jobs.deleteLocalPhoto(photo.id) }
    }

    fun templatesForCurrentStatus(): List<CustomerUpdateTemplate> =
        CustomerUpdateTemplates.forStatus(job.value?.status ?: JobStatus.Intake)

    fun updateContext(): CustomerUpdateContext {
        val detail = job.value
        return CustomerUpdateContext(
            customerFirstName = detail?.customer?.name?.trim()?.substringBefore(' ')?.ifBlank { "there" } ?: "there",
            deviceLabel = detail?.device?.label ?: "device",
            jobCode = detail?.jobCode ?: "",
        )
    }

    fun sendCustomerUpdate(body: String) {
        val detail = job.value ?: return
        val phone = detail.customer.phone
        if (phone.isNullOrBlank()) {
            _state.update { it.copy(error = "This customer has no phone number on file.") }
            return
        }
        viewModelScope.launch {
            jobs.sendCustomerUpdate(jobId, phone, body.trim(), detail.customer.id)
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "Update queued for ${detail.customer.name ?: "customer"}") }
        }
    }

    /** Authenticated attachment URL, for rendering server-side photos. */
    fun photoUrl(photo: JobPhoto): String? = photo.remoteId?.let {
        "${com.techlane.pos.BuildConfig.API_BASE.trimEnd('/')}/repairs/$jobId/attachments/$it/content"
    }

    private fun kick() = JobSyncWorker.enqueue(context)
}
