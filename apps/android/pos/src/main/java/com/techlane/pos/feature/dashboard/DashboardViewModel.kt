package com.techlane.pos.feature.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.repository.DashboardRepository
import com.techlane.pos.data.session.PosPreferences
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.DashboardData
import com.techlane.pos.domain.model.DashboardRules
import com.techlane.pos.domain.model.QuickAction
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.Calendar
import javax.inject.Inject

data class DashboardUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val error: String? = null,
    val prefs: PosPreferences = PosPreferences(),
) {
    val firstName: String
        get() = prefs.displayName?.trim()?.substringBefore(' ')?.takeIf { it.isNotBlank() } ?: "there"

    /** Intake creation is permissioned server-side; this only shapes the shortcut. */
    val canCreateIntake: Boolean
        get() = prefs.roles.any { it == "owner" || it == "manager" || it == "cashier" || it == "technician" }

    val quickActions: List<QuickAction> get() = DashboardRules.quickActions(canCreateIntake)
}

@HiltViewModel
class DashboardViewModel @Inject constructor(
    private val dashboard: DashboardRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val _state = MutableStateFlow(DashboardUiState())
    val state: StateFlow<DashboardUiState> = _state.asStateFlow()

    val data: StateFlow<DashboardData> = dashboard.observe()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), DashboardData())

    init {
        viewModelScope.launch {
            prefs.preferences.collect { preferences ->
                _state.update { it.copy(prefs = preferences) }
            }
        }
        refresh(initial = true)
    }

    fun refresh(initial: Boolean = false) {
        _state.update { it.copy(refreshing = !initial, loading = initial, error = null) }
        viewModelScope.launch {
            // Cached counts stay on screen through a failed refresh; the banner
            // says why they may be behind rather than replacing them with an error.
            dashboard.refresh().onFailure { error ->
                _state.update { it.copy(error = error.message) }
            }
            _state.update { it.copy(refreshing = false, loading = false) }
        }
    }

    fun dismissError() = _state.update { it.copy(error = null) }
}

/** Greeting by local clock. Shop hours run long, so evening starts late. */
fun greetingFor(hour: Int = Calendar.getInstance().get(Calendar.HOUR_OF_DAY)): String = when (hour) {
    in 0..4 -> "Good evening"
    in 5..11 -> "Good morning"
    in 12..16 -> "Good afternoon"
    else -> "Good evening"
}
