package com.techlane.pos.feature.sales

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.repository.SalesRepository
import com.techlane.pos.data.repository.epochDayFor
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.SaleSummary
import com.techlane.pos.domain.model.SalesFilter
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

/** Quick date chips — a full custom range picker is a small follow-up; the
 *  backend already accepts arbitrary from/to (ListSalesFilter), so this is a
 *  UI gap, not a capability gap. */
enum class DateQuickFilter(val label: String) { All("All"), Today("Today"), ThisWeek("This week") }

data class SalesHistoryUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val error: String? = null,
    val query: String = "",
    val method: String? = null,
    val status: String? = null,
    val dateFilter: DateQuickFilter = DateQuickFilter.All,
)

@HiltViewModel
class SalesHistoryViewModel @Inject constructor(
    private val salesRepo: SalesRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val _state = MutableStateFlow(SalesHistoryUiState())
    val state: StateFlow<SalesHistoryUiState> = _state.asStateFlow()

    private val _sales = MutableStateFlow<List<SaleSummary>>(emptyList())
    val sales: StateFlow<List<SaleSummary>> = _sales.asStateFlow()

    private var branchId: String? = null
    private var searchJob: Job? = null

    init {
        viewModelScope.launch {
            branchId = prefs.preferences.first().branchId
            load()
        }
    }

    fun setQuery(value: String) {
        _state.update { it.copy(query = value) }
        // Debounced — a keystroke-per-request search against ILIKE/EXISTS
        // subqueries is wasteful, and 300ms is imperceptible to type against.
        searchJob?.cancel()
        searchJob = viewModelScope.launch {
            delay(300)
            load()
        }
    }

    fun setMethod(value: String?) {
        _state.update { it.copy(method = if (it.method == value) null else value) }
        load()
    }

    fun setStatus(value: String?) {
        _state.update { it.copy(status = if (it.status == value) null else value) }
        load()
    }

    fun setDateFilter(value: DateQuickFilter) {
        _state.update { it.copy(dateFilter = if (it.dateFilter == value) DateQuickFilter.All else value) }
        load()
    }

    fun refresh() {
        _state.update { it.copy(refreshing = true) }
        load()
    }

    private fun load() {
        viewModelScope.launch {
            val s = _state.value
            _state.update { it.copy(loading = it.loading || !it.refreshing, error = null) }
            val fromEpochDay = when (s.dateFilter) {
                DateQuickFilter.All -> null
                DateQuickFilter.Today -> epochDayFor(0)
                DateQuickFilter.ThisWeek -> epochDayFor(6)
            }
            val filter = SalesFilter(
                query = s.query,
                method = s.method,
                status = s.status,
                fromEpochDay = fromEpochDay,
            )
            salesRepo.listSales(filter, branchId)
                .onSuccess { _sales.value = it }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not load sales") } }
            _state.update { it.copy(loading = false, refreshing = false) }
        }
    }
}
