package com.techlane.pos.feature.auth

import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.repository.AuthRepository
import com.techlane.pos.data.repository.LoginOutcome
import com.techlane.pos.data.security.BiometricCancelled
import com.techlane.pos.data.security.BiometricVault
import com.techlane.pos.data.session.PreferencesStore
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class LoginUiState(
    val email: String = "",
    val password: String = "",
    val mfaCode: String = "",
    val mfaChallenge: String? = null,
    val busy: Boolean = false,
    val error: String? = null,
    val biometricAvailable: Boolean = false,
    val biometricEnrolled: Boolean = false,
    val signedIn: Boolean = false,
    /** Shown once after a successful password login, offering fingerprint setup. */
    val offerBiometricEnrolment: Boolean = false,
) {
    val stage: LoginStage get() = if (mfaChallenge != null) LoginStage.Mfa else LoginStage.Password
    val canSubmit: Boolean
        get() = when (stage) {
            LoginStage.Password -> email.isNotBlank() && password.length >= 4 && !busy
            LoginStage.Mfa -> mfaCode.length >= 6 && !busy
        }
}

enum class LoginStage { Password, Mfa }

@HiltViewModel
class AuthViewModel @Inject constructor(
    private val auth: AuthRepository,
    private val vault: BiometricVault,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val _state = MutableStateFlow(LoginUiState())
    val state: StateFlow<LoginUiState> = _state.asStateFlow()

    init {
        refreshBiometricState()
    }

    private fun refreshBiometricState() {
        viewModelScope.launch {
            val enabled = prefs.preferences.first().biometricEnabled
            _state.update {
                it.copy(
                    biometricAvailable = vault.availability() == BiometricVault.Availability.Available,
                    biometricEnrolled = enabled && vault.hasStoredCredential(),
                )
            }
        }
    }

    fun onEmailChange(value: String) = _state.update { it.copy(email = value, error = null) }
    fun onPasswordChange(value: String) = _state.update { it.copy(password = value, error = null) }
    fun onMfaCodeChange(value: String) =
        _state.update { it.copy(mfaCode = value.filter(Char::isDigit).take(6), error = null) }

    fun backToPassword() = _state.update { it.copy(mfaChallenge = null, mfaCode = "", error = null) }

    fun submit() {
        val current = _state.value
        if (!current.canSubmit) return
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val result = when (current.stage) {
                LoginStage.Password -> auth.login(current.email, current.password)
                LoginStage.Mfa -> auth.verifyMfa(current.mfaChallenge!!, current.mfaCode)
            }
            result
                .onSuccess { outcome ->
                    when (outcome) {
                        is LoginOutcome.MfaRequired -> _state.update {
                            it.copy(busy = false, mfaChallenge = outcome.challenge, password = "")
                        }
                        is LoginOutcome.Success -> _state.update {
                            it.copy(
                                busy = false,
                                signedIn = true,
                                password = "",
                                mfaCode = "",
                                mfaChallenge = null,
                                // Only offer enrolment when the hardware can actually honour it.
                                offerBiometricEnrolment = it.biometricAvailable && !it.biometricEnrolled,
                            )
                        }
                    }
                }
                .onFailure { error ->
                    _state.update { it.copy(busy = false, error = error.message ?: "Sign-in failed") }
                }
        }
    }

    /** Fingerprint path: recover the refresh token, then trade it for a session. */
    fun unlockWithBiometrics(activity: FragmentActivity) {
        if (!_state.value.biometricEnrolled) return
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            vault.unlock(activity)
                .onSuccess { refreshToken ->
                    auth.resumeWithRefreshToken(refreshToken)
                        .onSuccess { _state.update { it.copy(busy = false, signedIn = true) } }
                        .onFailure { error ->
                            _state.update {
                                it.copy(
                                    busy = false,
                                    biometricEnrolled = false,
                                    error = error.message ?: "Fingerprint sign-in expired. Use your password.",
                                )
                            }
                        }
                }
                .onFailure { error ->
                    val cancelled = (error as? BiometricCancelled)?.userCancelled == true
                    _state.update {
                        it.copy(busy = false, error = if (cancelled) null else error.message)
                    }
                }
        }
    }

    /** Called from the post-login prompt. Never stores the password itself. */
    fun enableBiometrics(activity: FragmentActivity, onDone: () -> Unit) {
        val refreshToken = auth.currentRefreshToken()
        if (refreshToken == null) {
            _state.update { it.copy(offerBiometricEnrolment = false) }
            onDone()
            return
        }
        viewModelScope.launch {
            vault.enrol(activity, refreshToken)
                .onSuccess {
                    prefs.setBiometricEnabled(true)
                    _state.update { it.copy(offerBiometricEnrolment = false, biometricEnrolled = true) }
                }
                .onFailure { error ->
                    val cancelled = (error as? BiometricCancelled)?.userCancelled == true
                    _state.update {
                        it.copy(
                            offerBiometricEnrolment = false,
                            error = if (cancelled) null else error.message,
                        )
                    }
                }
            onDone()
        }
    }

    fun skipBiometricEnrolment(onDone: () -> Unit) {
        _state.update { it.copy(offerBiometricEnrolment = false) }
        onDone()
    }

    fun dismissError() = _state.update { it.copy(error = null) }
}
