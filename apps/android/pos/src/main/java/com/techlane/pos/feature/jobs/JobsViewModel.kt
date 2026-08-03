package com.techlane.pos.feature.jobs

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.JobFilter
import com.techlane.pos.domain.model.JobSummary
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class JobsUiState(
    val filter: JobFilter = JobFilter.Mine,
    val query: String = "",
    val loading: Boolean = false,
    val refreshing: Boolean = false,
    val error: String? = null,
    val pendingSync: Int = 0,
    val meId: String? = null,
    /** True on first open before any board has been cached. */
    val firstLoad: Boolean = true,
)

@OptIn(FlowPreview::class)
@HiltViewModel
class JobsViewModel @Inject constructor(
    savedStateHandle: androidx.lifecycle.SavedStateHandle,
    private val jobs: JobRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    /** Dashboard tiles deep-link straight into a queue. */
    private val initialFilter: JobFilter? =
        savedStateHandle.get<String>("filter")?.let { name ->
            JobFilter.entries.firstOrNull { it.name == name }
        }

    private val _state = MutableStateFlow(JobsUiState(filter = initialFilter ?: JobFilter.Mine))
    val state: StateFlow<JobsUiState> = _state.asStateFlow()

    private val query = MutableStateFlow("")

    /** Server-side hits for a query that reaches past what this phone has cached. */
    private val remoteMatches = MutableStateFlow<List<JobSummary>>(emptyList())

    val technicianNames: StateFlow<Map<String, String>> = jobs.observeTechnicians()
        .map { list -> list.associate { it.id to it.displayName } }
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyMap())

    val visibleJobs: StateFlow<List<JobSummary>> = combine(
        jobs.observeJobs(),
        _state.map { it.filter }.distinctUntilChanged(),
        query.debounce(300).distinctUntilChanged(),
        remoteMatches,
        _state.map { it.meId }.distinctUntilChanged(),
    ) { cached, filter, text, remote, meId ->
        // Remote hits are merged in, not swapped for: a technician searching for a
        // job they have open locally should still see their unsynced edits on it.
        val pool = (cached + remote.filter { r -> cached.none { it.id == r.id } })
        pool.filter { it.matches(filter, meId) && it.matches(text) }
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    init {
        viewModelScope.launch {
            jobs.observePendingSyncCount().collect { count ->
                _state.update { it.copy(pendingSync = count) }
            }
        }
        viewModelScope.launch {
            val preferences = prefs.preferences.first()
            _state.update { it.copy(meId = preferences.userId) }
            // An explicit deep-link wins; otherwise a technician's own bench is
            // the useful default, and a shop with no identity yet falls back to
            // the whole board rather than an empty one.
            when {
                initialFilter != null -> _state.update { it.copy(filter = initialFilter) }
                preferences.userId == null -> _state.update { it.copy(filter = JobFilter.All) }
            }
        }
        viewModelScope.launch {
            query.debounce(350).distinctUntilChanged().collect { text ->
                if (text.length < 3) {
                    remoteMatches.value = emptyList()
                    return@collect
                }
                jobs.searchJobs(text).onSuccess { remoteMatches.value = it }
            }
        }
        refresh(initial = true)
    }

    fun setFilter(filter: JobFilter) = _state.update { it.copy(filter = filter) }

    fun setQuery(value: String) {
        query.value = value
        _state.update { it.copy(query = value) }
    }

    fun refresh(initial: Boolean = false) {
        _state.update { it.copy(refreshing = !initial, loading = initial, error = null) }
        viewModelScope.launch {
            val branchId = prefs.preferences.first().branchId
            jobs.refreshTechnicians()
            jobs.refreshJobs(branchId = branchId)
                .onFailure { error ->
                    // A cached board is still worth showing; the banner explains
                    // why it may be behind rather than replacing it with an error.
                    _state.update { it.copy(error = error.message) }
                }
            _state.update { it.copy(refreshing = false, loading = false, firstLoad = false) }
        }
    }

    fun dismissError() = _state.update { it.copy(error = null) }

    private fun JobSummary.matches(filter: JobFilter, meId: String?): Boolean = when (filter) {
        JobFilter.Mine -> technicianId != null && technicianId == meId
        JobFilter.All -> true
        JobFilter.AwaitingApproval -> awaitingApproval && status.isOpen
        else -> filter.statusFilter?.let { status == it } ?: true
    }

    private fun JobSummary.matches(text: String): Boolean {
        if (text.isBlank()) return true
        val needle = text.trim().lowercase()
        return jobCode.lowercase().contains(needle) ||
            customerName?.lowercase()?.contains(needle) == true ||
            deviceLabel.lowercase().contains(needle) ||
            customerPhone?.filter(Char::isDigit)?.contains(needle.filter(Char::isDigit).ifBlank { "\u0000" }) == true ||
            imei?.lowercase()?.contains(needle) == true ||
            serialNumber?.lowercase()?.contains(needle) == true
    }
}
