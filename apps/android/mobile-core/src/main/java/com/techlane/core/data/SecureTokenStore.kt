package com.techlane.core.data

import android.content.Context
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/** Per-app encrypted token store. Pass a unique prefsName per applicationId. */
class SecureTokenStore(context: Context, prefsName: String) {
    private val appContext = context.applicationContext
    private val storeName = prefsName

    private val prefs = openPrefs()

    private fun openPrefs(): android.content.SharedPreferences {
        return try {
            createEncryptedPrefs()
        } catch (e: Exception) {
            // Keystore / prefs corruption after OS update or reinstall-over-update
            // otherwise crashes the app before the login screen can appear.
            Log.e("SecureTokenStore", "Encrypted prefs unreadable; resetting $storeName", e)
            runCatching {
                appContext.deleteSharedPreferences(storeName)
            }
            try {
                createEncryptedPrefs()
            } catch (e2: Exception) {
                Log.e("SecureTokenStore", "Falling back to plain prefs for $storeName", e2)
                appContext.getSharedPreferences("${storeName}_fallback", Context.MODE_PRIVATE)
            }
        }
    }

    private fun createEncryptedPrefs(): android.content.SharedPreferences {
        val masterKey = MasterKey.Builder(appContext)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        return EncryptedSharedPreferences.create(
            appContext,
            storeName,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    var accessToken: String?
        get() = prefs.getString(KEY_ACCESS, null)
        set(value) {
            prefs.edit().putString(KEY_ACCESS, value).commit()
        }

    var refreshToken: String?
        get() = prefs.getString(KEY_REFRESH, null)
        set(value) {
            prefs.edit().putString(KEY_REFRESH, value).commit()
        }

    var sessionToken: String?
        get() = prefs.getString(KEY_SESSION, null)
        set(value) {
            prefs.edit().putString(KEY_SESSION, value).commit()
        }

    var selectedBranchId: String?
        get() = prefs.getString(KEY_BRANCH, null)
        set(value) {
            prefs.edit().putString(KEY_BRANCH, value).apply()
        }

    var phone: String?
        get() = prefs.getString(KEY_PHONE, null)
        set(value) {
            prefs.edit().putString(KEY_PHONE, value).apply()
        }

    var displayName: String?
        get() = prefs.getString(KEY_NAME, null)
        set(value) {
            prefs.edit().putString(KEY_NAME, value).apply()
        }

    var deviceId: String
        get() {
            val existing = prefs.getString(KEY_DEVICE, null)
            if (existing != null) return existing
            val id = java.util.UUID.randomUUID().toString()
            prefs.edit().putString(KEY_DEVICE, id).commit()
            return id
        }
        set(value) {
            prefs.edit().putString(KEY_DEVICE, value).apply()
        }

    fun clearSession() {
        prefs.edit()
            .remove(KEY_ACCESS)
            .remove(KEY_REFRESH)
            .remove(KEY_SESSION)
            .remove(KEY_PHONE)
            .remove(KEY_NAME)
            .commit()
    }

    fun clear() {
        val device = prefs.getString(KEY_DEVICE, null)
        val branch = prefs.getString(KEY_BRANCH, null)
        prefs.edit().clear().commit()
        val restore = prefs.edit()
        if (device != null) restore.putString(KEY_DEVICE, device)
        if (branch != null) restore.putString(KEY_BRANCH, branch)
        restore.commit()
    }

    companion object {
        private const val KEY_ACCESS = "access"
        private const val KEY_REFRESH = "refresh"
        private const val KEY_SESSION = "session"
        private const val KEY_DEVICE = "device_id"
        private const val KEY_BRANCH = "selected_branch_id"
        private const val KEY_PHONE = "phone"
        private const val KEY_NAME = "display_name"
    }
}
