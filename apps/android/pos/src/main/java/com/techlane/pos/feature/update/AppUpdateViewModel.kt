package com.techlane.pos.feature.update

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.update.AppUpdateRepository
import com.techlane.pos.data.update.AvailableUpdate
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Drives the app-wide update prompt. Separate from every feature ViewModel on
 * purpose — Jobs, POS and Settings should not know an update system exists.
 */
@HiltViewModel
class AppUpdateViewModel @Inject constructor(
    private val updates: AppUpdateRepository,
) : ViewModel() {

    val prompt: StateFlow<AvailableUpdate?> = updates.promptable
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), null)

    val installedVersionName: String = updates.installedVersionName

    /**
     * Safe to call on every app resume: the repository throttles to one real
     * request per interval, so this is a no-op most of the time.
     */
    fun checkOnResume() {
        viewModelScope.launch { updates.check() }
    }

    fun dismiss(versionCode: Int) {
        viewModelScope.launch { updates.dismiss(versionCode) }
    }
}
