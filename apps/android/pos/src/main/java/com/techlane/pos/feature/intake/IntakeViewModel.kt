package com.techlane.pos.feature.intake

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.data.local.TechnicianEntity
import com.techlane.pos.data.printer.PrinterRepository
import com.techlane.pos.data.remote.dto.CustomerDto
import com.techlane.pos.data.remote.dto.IntakeRequest
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.DeviceKind
import com.techlane.pos.domain.model.IntakeAccessory
import com.techlane.pos.domain.model.PromiseOption
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.Instant
import java.util.UUID
import javax.inject.Inject

/** What the operator sees after the job is booked — never a blank form. */
data class IntakeSuccess(
    val jobId: String,
    val jobCode: String,
    val deviceLabel: String,
    val customerName: String,
    /** Null until the receipt has been sent somewhere, then a short outcome line. */
    val receiptStatus: String? = null,
    val printerConnected: Boolean = false,
)

data class IntakeUiState(
    val customerPhone: String = "",
    val customerName: String = "",
    val customerEmail: String = "",
    /** Set once an existing customer is picked — suppresses re-typing their details. */
    val matchedCustomer: CustomerDto? = null,
    val suggestions: List<CustomerDto> = emptyList(),
    val searching: Boolean = false,
    val anonymous: Boolean = false,
    val deviceKind: DeviceKind = DeviceKind.Phone,
    val brand: String = "",
    val model: String = "",
    val identifier: String = "",
    val problem: String = "",
    val accessories: Set<IntakeAccessory> = emptySet(),
    val conditionNote: String = "",
    val promise: PromiseOption? = null,
    val estimateLabour: String = "",
    val technicianId: String? = null,
    val saving: Boolean = false,
    val error: String? = null,
    val success: IntakeSuccess? = null,
    val branchId: String? = null,
) {
    /**
     * The backend hard-requires a branch and a problem summary. A named walk-in
     * also needs a way to be reached, so a name without a phone is refused here
     * rather than booking an uncontactable job.
     */
    val problemValid: Boolean get() = problem.trim().length >= 3

    val hasCustomer: Boolean
        get() = anonymous || matchedCustomer != null ||
            (customerName.isNotBlank() && customerPhone.isNotBlank())

    val canSave: Boolean get() = !saving && branchId != null && problemValid && hasCustomer

    /** Shown inline as the next thing to do, not as a wall of errors on submit. */
    val validationHint: String?
        get() = when {
            branchId == null -> "Pick a branch in Settings before booking a job."
            !hasCustomer -> "Add the customer's name and phone, or mark it a walk-in."
            !problemValid -> "Describe the fault so the technician knows what to look at."
            else -> null
        }

    /** Compact pre-submit summary shown above the sticky action. */
    val summaryCustomer: String
        get() = when {
            anonymous -> "Walk-in"
            matchedCustomer != null -> matchedCustomer.fullName
            customerName.isNotBlank() -> customerName
            else -> "—"
        }

    val summaryDevice: String
        get() = listOf(brand.trim(), model.trim())
            .filter { it.isNotBlank() }
            .joinToString(" ")
            .ifBlank { deviceKind.label }
}

/**
 * Counter intake. Optimised for the 30–60 second walk-in: phone number first
 * (which is how a returning customer is recognised), then device, then fault.
 * Everything else is optional and can be filled in from Job Details later.
 */
@OptIn(FlowPreview::class)
@HiltViewModel
class IntakeViewModel @Inject constructor(
    private val jobs: JobRepository,
    private val prefs: PreferencesStore,
    private val printers: PrinterRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(IntakeUiState())
    val state: StateFlow<IntakeUiState> = _state.asStateFlow()

    private val phoneQuery = MutableStateFlow("")

    /**
     * Stable across retries so a tap that times out and is retried cannot book
     * the same walk-in twice. Rotated only once a job has actually been created.
     */
    private var intakeKey: String = UUID.randomUUID().toString()

    val technicians: StateFlow<List<TechnicianEntity>> = jobs.observeTechnicians()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    init {
        viewModelScope.launch {
            _state.update { it.copy(branchId = prefs.preferences.first().branchId) }
        }
        viewModelScope.launch { jobs.refreshTechnicians() }
        // Debounced so typing a number is one lookup, not ten.
        viewModelScope.launch {
            phoneQuery.debounce(350).distinctUntilChanged().collect { text ->
                if (text.length < 3) {
                    _state.update { it.copy(suggestions = emptyList(), searching = false) }
                    return@collect
                }
                _state.update { it.copy(searching = true) }
                val hits = jobs.searchCustomers(text)
                _state.update { current ->
                    // Dropped if the operator has already picked someone or moved on.
                    if (current.matchedCustomer != null) current.copy(searching = false)
                    else current.copy(suggestions = hits, searching = false)
                }
            }
        }
    }

    fun setCustomerPhone(value: String) {
        val cleaned = value.filter { it.isDigit() || it == '+' }.take(15)
        phoneQuery.value = cleaned
        _state.update {
            // Editing the number means they are no longer the matched customer.
            it.copy(customerPhone = cleaned, matchedCustomer = null, error = null)
        }
    }

    fun selectCustomer(customer: CustomerDto) = _state.update {
        it.copy(
            matchedCustomer = customer,
            customerName = customer.fullName,
            customerPhone = customer.phone ?: it.customerPhone,
            customerEmail = customer.email.orEmpty(),
            suggestions = emptyList(),
            error = null,
        )
    }

    fun clearMatchedCustomer() = _state.update { it.copy(matchedCustomer = null) }

    fun setCustomerName(value: String) = _state.update { it.copy(customerName = value, error = null) }
    fun setCustomerEmail(value: String) = _state.update { it.copy(customerEmail = value) }
    fun setBrand(value: String) = _state.update { it.copy(brand = value) }
    fun setModel(value: String) = _state.update { it.copy(model = value) }
    fun setIdentifier(value: String) = _state.update { it.copy(identifier = value) }
    fun setConditionNote(value: String) = _state.update { it.copy(conditionNote = value) }
    fun setTechnician(id: String?) = _state.update { it.copy(technicianId = id) }
    fun setPromise(option: PromiseOption?) = _state.update { it.copy(promise = option) }
    fun clearError() = _state.update { it.copy(error = null) }

    fun setEstimateLabour(value: String) =
        _state.update { it.copy(estimateLabour = value.filter(Char::isDigit)) }

    /** Switching device type re-scopes the quick issue chips, so clear stale ones. */
    fun setDeviceKind(kind: DeviceKind) = _state.update { it.copy(deviceKind = kind) }

    fun setProblem(value: String) = _state.update { it.copy(problem = value, error = null) }

    /** Chips append to the free text rather than replacing it — both are useful. */
    fun toggleIssue(issue: String) = _state.update { current ->
        val existing = current.problem.trim()
        val next = when {
            existing.isBlank() -> issue
            existing.contains(issue, ignoreCase = true) -> existing
            else -> "$existing, $issue"
        }
        current.copy(problem = next, error = null)
    }

    fun toggleAccessory(accessory: IntakeAccessory) = _state.update { current ->
        val next = current.accessories.toMutableSet()
        if (!next.add(accessory)) next.remove(accessory)
        current.copy(accessories = next)
    }

    fun setAnonymous(value: Boolean) = _state.update {
        it.copy(anonymous = value, matchedCustomer = null, suggestions = emptyList(), error = null)
    }

    fun save() {
        val current = _state.value
        if (!current.canSave) return
        val branchId = current.branchId ?: return
        _state.update { it.copy(saving = true, error = null) }
        viewModelScope.launch {
            jobs.createIntake(current.toRequest(branchId), intakeKey)
                .onSuccess { jobId -> onCreated(jobId, current) }
                .onFailure { error ->
                    _state.update { it.copy(saving = false, error = error.message ?: "Could not book the job.") }
                }
        }
    }

    /**
     * The receipt is part of a successful intake, not a follow-up chore: the
     * job is created, then the slip is printed if a printer is set up and
     * auto-print is on. Printing never gates the result — the job exists
     * either way, and the success sheet always offers Print and Share.
     */
    private suspend fun onCreated(jobId: String, submitted: IntakeUiState) {
        // The walk-in is booked; a later tap must start a genuinely new job.
        intakeKey = UUID.randomUUID().toString()
        val jobCode = jobs.observeJob(jobId).first()?.jobCode ?: ""
        val printerReady = printers.preferences.first().isConfigured
        _state.update {
            it.copy(
                saving = false,
                success = IntakeSuccess(
                    jobId = jobId,
                    jobCode = jobCode,
                    deviceLabel = submitted.summaryDevice,
                    customerName = submitted.summaryCustomer,
                    printerConnected = printerReady,
                    receiptStatus = if (printerReady) null else "Receipt ready — printer not connected",
                ),
            )
        }
        printers.autoPrintIntakeSlipIfEnabled(jobId) { outcome ->
            _state.update { current ->
                current.copy(success = current.success?.copy(receiptStatus = outcome))
            }
        }
    }

    /** Explicit Print from the success sheet. */
    fun printReceipt() {
        val jobId = _state.value.success?.jobId ?: return
        _state.update { it.copy(success = it.success?.copy(receiptStatus = "Printing…")) }
        viewModelScope.launch {
            val outcome = printers.printIntakeSlip(jobId)
                .fold({ "Receipt sent to the printer" }, { it.message ?: "Could not print the receipt" })
            _state.update { it.copy(success = it.success?.copy(receiptStatus = outcome)) }
        }
    }

    /** Clears the form for the next customer, keeping nothing from the last one. */
    fun startAnother() {
        phoneQuery.value = ""
        _state.update { IntakeUiState(branchId = it.branchId) }
    }
}

/**
 * One identifier field on screen rather than two: a phone's IMEI and a laptop's
 * serial occupy the same slot in a technician's head, and asking for both
 * guarantees one is left blank.
 */
internal fun IntakeUiState.toRequest(branchId: String) = IntakeRequest(
    branchId = branchId,
    problemSummary = problem.trim(),
    deviceKind = deviceKind.wire,
    anonymous = anonymous,
    // A matched customer is referenced by id so intake never creates a duplicate
    // record for someone the shop already knows.
    customerId = matchedCustomer?.id.takeIf { !anonymous },
    customerName = customerName.trim().takeIf { !anonymous && matchedCustomer == null && it.isNotBlank() },
    customerPhone = Msisdn.normalise(customerPhone).takeIf { !anonymous && matchedCustomer == null }
        ?: customerPhone.trim().takeIf { !anonymous && matchedCustomer == null && it.isNotBlank() },
    brand = brand.trim().takeIf { it.isNotBlank() },
    model = model.trim().takeIf { it.isNotBlank() },
    imei = identifier.trim().takeIf { it.isNotBlank() && deviceKind.identifierIsImei },
    serialNumber = identifier.trim().takeIf { it.isNotBlank() && !deviceKind.identifierIsImei },
    conditionTags = accessories.map { it.wire } +
        listOfNotNull(conditionNote.trim().takeIf { it.isNotBlank() }),
    estimateLaborAmount = estimateLabour.toDoubleOrNull()?.takeIf { it > 0 },
    technicianId = technicianId,
    promisedBy = promise?.at()?.let { Instant.ofEpochMilli(it).toString() },
)
