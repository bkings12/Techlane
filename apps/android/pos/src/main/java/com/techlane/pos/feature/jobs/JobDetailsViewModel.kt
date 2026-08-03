package com.techlane.pos.feature.jobs

import android.content.Context
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.data.local.TechnicianEntity
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
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.VerbalApprovalChannel
import com.techlane.pos.sync.JobSyncWorker
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.ExperimentalCoroutinesApi
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
import javax.inject.Inject

/** Which secondary surface, if any, is open over the detail screen. */
enum class JobSheet { None, Status, Technician, Diagnosis, Estimate, Approval, Parts, CustomerUpdate, Photo }

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
)

@OptIn(ExperimentalCoroutinesApi::class)
@HiltViewModel
class JobDetailsViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val jobs: JobRepository,
    private val shop: ShopRepository,
    private val prefs: PreferencesStore,
    @ApplicationContext private val context: Context,
) : ViewModel() {

    val jobId: String = checkNotNull(savedStateHandle["jobId"]) { "jobId is required" }

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
            _state.update { it.copy(meId = preferences.userId, locationId = preferences.locationId) }
        }
        refresh(initial = true)
    }

    fun refresh(initial: Boolean = false) {
        _state.update { it.copy(refreshing = !initial, loading = initial && job.value == null, error = null) }
        viewModelScope.launch {
            jobs.refreshJob(jobId).onFailure { error ->
                // Cached detail is still the technician's working copy.
                _state.update { it.copy(error = error.message) }
            }
            _state.update { it.copy(refreshing = false, loading = false) }
        }
    }

    fun openSheet(sheet: JobSheet) = _state.update { it.copy(sheet = sheet, message = null) }
    fun closeSheet() = _state.update { it.copy(sheet = JobSheet.None) }
    fun clearFeedback() = _state.update { it.copy(message = null, error = null) }
    fun setPartsQuery(value: String) = _state.update { it.copy(partsQuery = value) }
    fun setPendingPhotoKind(kind: PhotoKind) = _state.update { it.copy(pendingPhotoKind = kind) }

    // ------------------------------------------------------------- mutations
    //
    // Every one of these writes to Room first and queues the network call, so a
    // technician in a back room sees their own change immediately. `kick()` only
    // asks WorkManager to try; it never blocks the UI on the result.

    fun changeStatus(status: JobStatus, note: String?, closureReason: ClosureReason?) {
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

    fun addPart(item: CatalogItemEntity, quantity: Int) {
        val locationId = _state.value.locationId
        if (locationId.isNullOrBlank()) {
            _state.update { it.copy(error = "Pick a stock location in Settings before adding parts.") }
            return
        }
        viewModelScope.launch {
            jobs.addPart(
                jobId = jobId,
                variantId = item.variantId,
                locationId = locationId,
                name = item.productName,
                sku = item.sku,
                unitPrice = item.sellPrice,
                quantity = quantity,
            )
            kick()
            _state.update { it.copy(sheet = JobSheet.None, message = "${item.productName} added") }
        }
    }

    fun removePart(part: JobPart) {
        viewModelScope.launch {
            jobs.removePart(jobId, part)
            kick()
            _state.update { it.copy(message = "${part.name} removed") }
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
