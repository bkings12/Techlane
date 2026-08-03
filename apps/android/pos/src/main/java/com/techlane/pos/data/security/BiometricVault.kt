package com.techlane.pos.data.security

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricPrompt
import androidx.fragment.app.FragmentActivity
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.suspendCancellableCoroutine
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume

/**
 * Biometric sign-in without ever storing a password or PIN.
 *
 * On enrolment we encrypt the *refresh token* with an AES key that lives in the
 * Android Keystore and is marked `setUserAuthenticationRequired(true)`. The
 * ciphertext is useless without a fingerprint touch, and the key is invalidated
 * by the OS if the user adds a new fingerprint — which is exactly the behaviour
 * we want for a till that several staff share a room with.
 */
@Singleton
class BiometricVault @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    enum class Availability { Available, NoHardware, NotEnrolled, TemporarilyUnavailable }

    fun availability(): Availability =
        when (BiometricManager.from(context).canAuthenticate(BIOMETRIC_STRONG)) {
            BiometricManager.BIOMETRIC_SUCCESS -> Availability.Available
            BiometricManager.BIOMETRIC_ERROR_NONE_ENROLLED -> Availability.NotEnrolled
            BiometricManager.BIOMETRIC_ERROR_NO_HARDWARE,
            BiometricManager.BIOMETRIC_ERROR_SECURITY_UPDATE_REQUIRED,
            -> Availability.NoHardware
            else -> Availability.TemporarilyUnavailable
        }

    fun hasStoredCredential(): Boolean = prefs().contains(KEY_PAYLOAD)

    fun clear() {
        prefs().edit().remove(KEY_PAYLOAD).remove(KEY_IV).apply()
        runCatching { keyStore().deleteEntry(KEY_ALIAS) }
    }

    /**
     * Encrypts [refreshToken] behind a fresh biometric touch. Returns false if
     * the user backed out or the key could not be created.
     */
    suspend fun enrol(activity: FragmentActivity, refreshToken: String): Result<Unit> = runCatching {
        val cipher = Cipher.getInstance(TRANSFORMATION).apply {
            init(Cipher.ENCRYPT_MODE, getOrCreateKey())
        }
        val authenticated = prompt(
            activity = activity,
            cipher = cipher,
            title = "Turn on fingerprint sign-in",
            subtitle = "Confirm it's you so this phone can unlock the till next time.",
        ).getOrThrow()
        val encrypted = authenticated.doFinal(refreshToken.toByteArray(Charsets.UTF_8))
        prefs().edit()
            .putString(KEY_PAYLOAD, Base64.encodeToString(encrypted, Base64.NO_WRAP))
            .putString(KEY_IV, Base64.encodeToString(authenticated.iv, Base64.NO_WRAP))
            .apply()
    }

    /** Recovers the refresh token after a successful fingerprint touch. */
    suspend fun unlock(activity: FragmentActivity): Result<String> = runCatching {
        val payload = prefs().getString(KEY_PAYLOAD, null)
            ?: error("Fingerprint sign-in is not set up on this phone.")
        val iv = prefs().getString(KEY_IV, null)
            ?: error("Fingerprint sign-in is not set up on this phone.")
        val cipher = Cipher.getInstance(TRANSFORMATION).apply {
            init(
                Cipher.DECRYPT_MODE,
                existingKey() ?: error("Fingerprint sign-in needs to be set up again."),
                GCMParameterSpec(GCM_TAG_BITS, Base64.decode(iv, Base64.NO_WRAP)),
            )
        }
        val authenticated = prompt(
            activity = activity,
            cipher = cipher,
            title = "Unlock TechLane POS",
            subtitle = "Touch the fingerprint sensor to open the till.",
        ).getOrThrow()
        String(authenticated.doFinal(Base64.decode(payload, Base64.NO_WRAP)), Charsets.UTF_8)
    }.onFailure { error ->
        // A new fingerprint enrolment invalidates the key; wipe so the UI can
        // fall back to password rather than looping on a dead ciphertext.
        if (error is android.security.keystore.KeyPermanentlyInvalidatedException) clear()
    }

    private suspend fun prompt(
        activity: FragmentActivity,
        cipher: Cipher,
        title: String,
        subtitle: String,
    ): Result<Cipher> = suspendCancellableCoroutine { cont ->
        val executor = androidx.core.content.ContextCompat.getMainExecutor(activity)
        val prompt = BiometricPrompt(
            activity,
            executor,
            object : BiometricPrompt.AuthenticationCallback() {
                override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                    val crypto = result.cryptoObject?.cipher
                    if (crypto == null) {
                        if (cont.isActive) cont.resume(Result.failure(IllegalStateException("Biometric result had no cipher")))
                    } else if (cont.isActive) {
                        cont.resume(Result.success(crypto))
                    }
                }

                override fun onAuthenticationError(code: Int, message: CharSequence) {
                    if (cont.isActive) cont.resume(Result.failure(BiometricCancelled(code, message.toString())))
                }
                // onAuthenticationFailed (a non-matching finger) is not terminal —
                // the system prompt stays up and the user tries again.
            },
        )
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle(title)
            .setSubtitle(subtitle)
            .setNegativeButtonText("Use password")
            .setAllowedAuthenticators(BIOMETRIC_STRONG)
            .setConfirmationRequired(false)
            .build()
        prompt.authenticate(info, BiometricPrompt.CryptoObject(cipher))
        cont.invokeOnCancellation { prompt.cancelAuthentication() }
    }

    private fun prefs() = context.getSharedPreferences("techlane_pos_biometric", Context.MODE_PRIVATE)

    private fun keyStore() = KeyStore.getInstance(KEYSTORE).apply { load(null) }

    private fun existingKey(): SecretKey? =
        runCatching { keyStore().getKey(KEY_ALIAS, null) as? SecretKey }.getOrNull()

    private fun getOrCreateKey(): SecretKey = existingKey() ?: KeyGenerator
        .getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        .apply {
            init(
                KeyGenParameterSpec.Builder(
                    KEY_ALIAS,
                    KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                )
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setUserAuthenticationRequired(true)
                    .setInvalidatedByBiometricEnrollment(true)
                    .build(),
            )
        }
        .generateKey()

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val KEY_ALIAS = "techlane_pos_biometric_key"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val GCM_TAG_BITS = 128
        const val KEY_PAYLOAD = "payload"
        const val KEY_IV = "iv"
    }
}

/** Raised when the user dismissed the system prompt or the OS refused it. */
class BiometricCancelled(val code: Int, override val message: String) : Exception(message) {
    val userCancelled: Boolean
        get() = code == BiometricPrompt.ERROR_NEGATIVE_BUTTON ||
            code == BiometricPrompt.ERROR_USER_CANCELED ||
            code == BiometricPrompt.ERROR_CANCELED
}
