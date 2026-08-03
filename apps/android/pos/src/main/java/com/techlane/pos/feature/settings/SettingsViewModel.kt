package com.techlane.pos.feature.settings

import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.remote.dto.BranchDto
import com.techlane.pos.data.remote.dto.StockLocationDto
import com.techlane.pos.data.repository.AuthRepository
import com.techlane.pos.data.repository.ShopRepository
import com.techlane.pos.data.security.BiometricCancelled
import com.techlane.pos.data.security.BiometricVault
import com.techlane.pos.data.session.PosPreferences
import com.techlane.pos.data.session.PreferencesStore
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class SettingsUiState(
    val prefs: PosPreferences = PosPreferences(),
    val branches: List<BranchDto> = emptyList(),
    val locations: List<StockLocationDto> = emptyList(),
    val loading: Boolean = false,
    val syncing: Boolean = false,
    val message: String? = null,
    val error: String? = null,
    val biometricAvailable: Boolean = false,
    val catalogCount: Int? = null,
)

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val shop: ShopRepository,
    private val auth: AuthRepository,
    private val prefs: PreferencesStore,
    private val vault: BiometricVault,
) : ViewModel() {

    private val _state = MutableStateFlow(SettingsUiState())
    val state: StateFlow<SettingsUiState> = _state.asStateFlow()

    init {
        _state.update { it.copy(biometricAvailable = vault.availability() == BiometricVault.Availability.Available) }
        viewModelScope.launch {
            prefs.preferences.collect { preferences ->
                _state.update { it.copy(prefs = preferences) }
            }
        }
        loadBranches()
    }

    fun loadBranches() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            shop.branches()
                .onSuccess { branches ->
                    _state.update { it.copy(branches = branches, loading = false) }
                    // A single-branch shop shouldn't have to choose anything.
                    val current = prefs.preferences.first()
                    if (current.branchId == null && branches.size == 1) {
                        selectBranch(branches.first())
                    } else if (current.branchId != null) {
                        loadLocations(current.branchId)
                    }
                }
                .onFailure { error -> _state.update { it.copy(loading = false, error = error.message) } }
        }
    }

    fun selectBranch(branch: BranchDto) {
        viewModelScope.launch {
            prefs.setBranch(branch.id, branch.name)
            prefs.setLocation(null, null)
            loadLocations(branch.id)
        }
    }

    private fun loadLocations(branchId: String) {
        viewModelScope.launch {
            shop.locations(branchId)
                .onSuccess { locations ->
                    _state.update { it.copy(locations = locations) }
                    val current = prefs.preferences.first()
                    if (current.locationId == null) {
                        // Prefer the shop-floor location over a warehouse when obvious.
                        val preferred = locations.firstOrNull { it.locationType == "shop" }
                            ?: locations.firstOrNull()
                        preferred?.let { selectLocation(it) }
                    }
                }
                .onFailure { error -> _state.update { it.copy(error = error.message) } }
        }
    }

    fun selectLocation(location: StockLocationDto) {
        viewModelScope.launch {
            prefs.setLocation(location.id, location.name)
            syncCatalog()
        }
    }

    fun syncCatalog() {
        viewModelScope.launch {
            val locationId = prefs.preferences.first().locationId ?: return@launch
            _state.update { it.copy(syncing = true, error = null, message = null) }
            shop.syncCatalog(locationId)
                .onSuccess { count ->
                    _state.update { it.copy(syncing = false, catalogCount = count, message = "$count items cached for offline use") }
                }
                .onFailure { error -> _state.update { it.copy(syncing = false, error = error.message) } }
        }
    }

    fun setBiometricEnabled(activity: FragmentActivity, enabled: Boolean) {
        viewModelScope.launch {
            if (!enabled) {
                vault.clear()
                prefs.setBiometricEnabled(false)
                _state.update { it.copy(message = "Fingerprint sign-in turned off") }
                return@launch
            }
            val refreshToken = auth.currentRefreshToken()
            if (refreshToken == null) {
                _state.update { it.copy(error = "Sign in again before turning on fingerprint sign-in.") }
                return@launch
            }
            vault.enrol(activity, refreshToken)
                .onSuccess {
                    prefs.setBiometricEnabled(true)
                    _state.update { it.copy(message = "Fingerprint sign-in is on") }
                }
                .onFailure { error ->
                    val cancelled = (error as? BiometricCancelled)?.userCancelled == true
                    if (!cancelled) _state.update { it.copy(error = error.message) }
                }
        }
    }

    fun signOut(onDone: () -> Unit) {
        viewModelScope.launch {
            auth.signOut()
            onDone()
        }
    }

    fun clearFeedback() = _state.update { it.copy(message = null, error = null) }
}
