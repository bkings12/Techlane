package com.techlane.pos.feature.intake

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.local.TechnicianEntity
import com.techlane.pos.data.remote.dto.IntakeRequest
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.DeviceKind
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class IntakeUiState(
    val customerName: String = "",
    val customerPhone: String = "",
    val anonymous: Boolean = false,
    val deviceKind: DeviceKind = DeviceKind.Phone,
    val brand: String = "",
    val model: String = "",
    val identifier: String = "",
    val problem: String = "",
    val estimateLabour: String = "",
    val technicianId: String? = null,
    val saving: Boolean = false,
    val error: String? = null,
    /** Set once the server has the job; the screen navigates on this. */
    val createdJobId: String? = null,
    val branchId: String? = null,
) {
    /**
     * The backend only hard-requires a branch and a problem summary. A named
     * walk-in also needs a way to reach them, which is why a name without a
     * phone is refused here rather than silently booking an uncontactable job.
     */
    val problemValid: Boolean get() = problem.trim().length >= 3

    val customerValid: Boolean
        get() = anonymous || (customerName.isNotBlank() && customerPhone.isNotBlank())

    val canSave: Boolean get() = !saving && branchId != null && problemValid && customerValid

    val validationHint: String?
        get() = when {
            branchId == null -> "Pick a branch in Settings before booking a job."
            !customerValid -> "Add the customer's name and phone, or mark it a walk-in."
            !problemValid -> "Describe the fault so the technician knows what to look at."
            else -> null
        }
}

/**
 * Counter intake. Kept deliberately short — a queue at the counter is the
 * enemy, so this asks only for what the shop cannot reconstruct later:
 * who, what device, and what is wrong with it. Everything else (diagnosis,
 * parts, estimates, promise dates) is added from Job Details afterwards.
 */
@HiltViewModel
class IntakeViewModel @Inject constructor(
    private val jobs: JobRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val _state = MutableStateFlow(IntakeUiState())
    val state: StateFlow<IntakeUiState> = _state.asStateFlow()

    val technicians: StateFlow<List<TechnicianEntity>> = jobs.observeTechnicians()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    init {
        viewModelScope.launch {
            val preferences = prefs.preferences.first()
            _state.update { it.copy(branchId = preferences.branchId) }
        }
        viewModelScope.launch { jobs.refreshTechnicians() }
    }

    fun setCustomerName(value: String) = _state.update { it.copy(customerName = value, error = null) }
    fun setCustomerPhone(value: String) = _state.update { it.copy(customerPhone = value, error = null) }
    fun setDeviceKind(kind: DeviceKind) = _state.update { it.copy(deviceKind = kind) }
    fun setBrand(value: String) = _state.update { it.copy(brand = value) }
    fun setModel(value: String) = _state.update { it.copy(model = value) }
    fun setIdentifier(value: String) = _state.update { it.copy(identifier = value) }
    fun setProblem(value: String) = _state.update { it.copy(problem = value, error = null) }
    fun setEstimateLabour(value: String) =
        _state.update { it.copy(estimateLabour = value.filter(Char::isDigit)) }

    fun setTechnician(id: String?) = _state.update { it.copy(technicianId = id) }

    fun clearError() = _state.update { it.copy(error = null) }

    /** Walk-in: no name or number taken, so the slip's pickup code is the only claim. */
    fun setAnonymous(value: Boolean) = _state.update {
        it.copy(anonymous = value, error = null)
    }

    fun save() {
        val current = _state.value
        if (!current.canSave) return
        val branchId = current.branchId ?: return
        _state.update { it.copy(saving = true, error = null) }
        viewModelScope.launch {
            jobs.createIntake(current.toRequest(branchId))
                .onSuccess { jobId -> _state.update { it.copy(saving = false, createdJobId = jobId) } }
                .onFailure { error ->
                    _state.update { it.copy(saving = false, error = error.message ?: "Could not book the job.") }
                }
        }
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
    customerName = customerName.trim().takeIf { !anonymous && it.isNotBlank() },
    customerPhone = customerPhone.trim().takeIf { !anonymous && it.isNotBlank() },
    brand = brand.trim().takeIf { it.isNotBlank() },
    model = model.trim().takeIf { it.isNotBlank() },
    imei = identifier.trim().takeIf { it.isNotBlank() && deviceKind.identifierIsImei },
    serialNumber = identifier.trim().takeIf { it.isNotBlank() && !deviceKind.identifierIsImei },
    estimateLaborAmount = estimateLabour.toDoubleOrNull()?.takeIf { it > 0 },
    technicianId = technicianId,
)
