package com.techlane.pos.data.session

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Access/refresh tokens at rest. EncryptedSharedPreferences keeps them out of a
 * plain-text prefs file; the biometric layer adds a second, user-presence-bound
 * copy of the refresh token (see BiometricVault).
 *
 * Reads are synchronous on purpose: the OkHttp interceptor needs the current
 * access token on a background thread without suspending.
 */
@Singleton
class SecureTokenStore @Inject constructor(
    @ApplicationContext context: Context,
) {
    private val prefs: SharedPreferences = runCatching {
        val key = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        EncryptedSharedPreferences.create(
            context,
            "techlane_pos_session",
            key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        ) as SharedPreferences
    }.getOrElse {
        // Keystore can be unusable on a handful of OEM builds. Falling back keeps
        // the shop running rather than bricking the app on that device.
        context.getSharedPreferences("techlane_pos_session_fallback", Context.MODE_PRIVATE)
    }

    private val _signedIn = MutableStateFlow(prefs.getString(KEY_REFRESH, null) != null)
    val signedIn: StateFlow<Boolean> = _signedIn.asStateFlow()

    var accessToken: String?
        get() = prefs.getString(KEY_ACCESS, null)
        set(value) = prefs.edit().putString(KEY_ACCESS, value).apply()

    var refreshToken: String?
        get() = prefs.getString(KEY_REFRESH, null)
        set(value) {
            prefs.edit().putString(KEY_REFRESH, value).apply()
            _signedIn.value = value != null
        }

    /** Stable per-install id used as the device fingerprint for sync/registration. */
    val deviceId: String
        get() = prefs.getString(KEY_DEVICE_ID, null) ?: UUID.randomUUID().toString().also {
            prefs.edit().putString(KEY_DEVICE_ID, it).apply()
        }

    fun save(access: String, refresh: String) {
        prefs.edit().putString(KEY_ACCESS, access).putString(KEY_REFRESH, refresh).apply()
        _signedIn.value = true
    }

    fun clear() {
        prefs.edit().remove(KEY_ACCESS).remove(KEY_REFRESH).apply()
        _signedIn.value = false
    }

    private companion object {
        const val KEY_ACCESS = "access_token"
        const val KEY_REFRESH = "refresh_token"
        const val KEY_DEVICE_ID = "device_id"
    }
}
