package com.techlane.pos.data.repository

import com.techlane.pos.data.local.PosDatabase
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.LoginRequest
import com.techlane.pos.data.remote.dto.MeDto
import com.techlane.pos.data.remote.dto.MfaVerifyRequest
import com.techlane.pos.data.remote.dto.RefreshRequest
import com.techlane.pos.data.remote.toAppException
import com.techlane.pos.data.security.BiometricVault
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.data.session.SecureTokenStore
import kotlinx.coroutines.flow.StateFlow
import javax.inject.Inject
import javax.inject.Singleton

/** What a password login produced — tokens, or a second factor still to clear. */
sealed interface LoginOutcome {
    data class Success(val profile: MeDto?) : LoginOutcome
    data class MfaRequired(val challenge: String) : LoginOutcome
}

@Singleton
class AuthRepository @Inject constructor(
    private val api: TechLaneApi,
    private val tokens: SecureTokenStore,
    private val prefs: PreferencesStore,
    private val vault: BiometricVault,
    private val database: PosDatabase,
) {
    val signedIn: StateFlow<Boolean> = tokens.signedIn

    suspend fun login(email: String, password: String): Result<LoginOutcome> = runCatching {
        val response = api.login(LoginRequest(email.trim().lowercase(), password))
        if (response.mfaRequired) {
            val challenge = response.mfaChallenge
                ?: error("This account needs a verification code, but the server did not send one.")
            return@runCatching LoginOutcome.MfaRequired(challenge)
        }
        val pair = response.tokenPair() ?: error("Sign-in failed: the server did not return a session.")
        tokens.save(pair.accessToken, pair.refreshToken)
        LoginOutcome.Success(loadProfile())
    }.recoverCatching { throw it.toAppException() }

    suspend fun verifyMfa(challenge: String, code: String): Result<LoginOutcome.Success> = runCatching {
        val pair = api.verifyMfa(MfaVerifyRequest(challenge, code.trim()))
        tokens.save(pair.accessToken, pair.refreshToken)
        LoginOutcome.Success(loadProfile())
    }.recoverCatching { throw it.toAppException() }

    /**
     * Exchanges a biometric-recovered refresh token for a live session. The
     * fingerprint proves presence; the server still decides whether the session
     * is valid, so a revoked staff account cannot get back in with a stale key.
     */
    suspend fun resumeWithRefreshToken(refreshToken: String): Result<Unit> = runCatching {
        val pair = api.refresh(RefreshRequest(refreshToken))
        tokens.save(pair.accessToken, pair.refreshToken)
        loadProfile()
        Unit
    }.recoverCatching {
        // A rejected token means the biometric copy is worthless — drop it so the
        // technician is not stuck touching a sensor that can never work.
        vault.clear()
        prefs.setBiometricEnabled(false)
        throw it.toAppException()
    }

    suspend fun refreshProfile(): Result<MeDto> = runCatching {
        loadProfile() ?: error("Could not load your profile")
    }.recoverCatching { throw it.toAppException() }

    private suspend fun loadProfile(): MeDto? = runCatching {
        api.me().also {
            prefs.setDisplayName(it.displayName.ifBlank { it.email })
            prefs.setRoles(it.roles)
            prefs.setUserId(it.id)
        }
    }.getOrNull()

    /** Current refresh token, for handing to the vault at enrolment time. */
    fun currentRefreshToken(): String? = tokens.refreshToken

    suspend fun signOut() {
        tokens.clear()
        vault.clear()
        prefs.clearSessionScoped()
        runCatching { database.clearAllTables() }
    }
}
