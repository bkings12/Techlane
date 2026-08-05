package com.techlane.pos.data.session

import android.content.Context
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.dataStore by preferencesDataStore(name = "techlane_pos_prefs")

/** Non-secret preferences: where this till is stationed, and how it looks. */
@Singleton
class PreferencesStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    private val store = context.dataStore

    val preferences: Flow<PosPreferences> = store.data.map { it.toPosPreferences() }

    suspend fun setBranch(branchId: String?, branchName: String?) = edit {
        it.putOrRemove(KEY_BRANCH_ID, branchId)
        it.putOrRemove(KEY_BRANCH_NAME, branchName)
    }

    suspend fun setLocation(locationId: String?, locationName: String?) = edit {
        it.putOrRemove(KEY_LOCATION_ID, locationId)
        it.putOrRemove(KEY_LOCATION_NAME, locationName)
    }

    suspend fun setBiometricEnabled(enabled: Boolean) = edit { it[KEY_BIOMETRIC] = enabled }

    suspend fun setDisplayName(name: String?) = edit { it.putOrRemove(KEY_DISPLAY_NAME, name) }

    suspend fun setUserId(id: String?) = edit { it.putOrRemove(KEY_USER_ID, id) }

    suspend fun setRoles(roles: List<String>) = edit { it[KEY_ROLES] = roles.joinToString(",") }

    suspend fun setLastPhone(phone: String?) = edit { it.putOrRemove(KEY_LAST_PHONE, phone) }

    suspend fun clearSessionScoped() = edit {
        it.remove(KEY_DISPLAY_NAME)
        it.remove(KEY_USER_ID)
        it.remove(KEY_ROLES)
        it.remove(KEY_LAST_PHONE)
        it.remove(KEY_BIOMETRIC)
    }

    private suspend fun edit(block: (androidx.datastore.preferences.core.MutablePreferences) -> Unit) {
        store.edit(block)
    }

    private fun androidx.datastore.preferences.core.MutablePreferences.putOrRemove(
        key: Preferences.Key<String>,
        value: String?,
    ) {
        if (value.isNullOrBlank()) remove(key) else set(key, value)
    }

    private fun Preferences.toPosPreferences() = PosPreferences(
        branchId = this[KEY_BRANCH_ID],
        branchName = this[KEY_BRANCH_NAME],
        locationId = this[KEY_LOCATION_ID],
        locationName = this[KEY_LOCATION_NAME],
        biometricEnabled = this[KEY_BIOMETRIC] ?: false,
        displayName = this[KEY_DISPLAY_NAME],
        userId = this[KEY_USER_ID],
        roles = this[KEY_ROLES]?.split(",")?.filter { it.isNotBlank() } ?: emptyList(),
        lastPhone = this[KEY_LAST_PHONE],
    )

    private companion object {
        val KEY_BRANCH_ID = stringPreferencesKey("branch_id")
        val KEY_BRANCH_NAME = stringPreferencesKey("branch_name")
        val KEY_LOCATION_ID = stringPreferencesKey("location_id")
        val KEY_LOCATION_NAME = stringPreferencesKey("location_name")
        val KEY_BIOMETRIC = booleanPreferencesKey("biometric_enabled")
        val KEY_DISPLAY_NAME = stringPreferencesKey("display_name")
        val KEY_USER_ID = stringPreferencesKey("user_id")
        val KEY_ROLES = stringPreferencesKey("roles")
        val KEY_LAST_PHONE = stringPreferencesKey("last_phone")
    }
}

data class PosPreferences(
    val branchId: String? = null,
    val branchName: String? = null,
    val locationId: String? = null,
    val locationName: String? = null,
    val biometricEnabled: Boolean = false,
    val displayName: String? = null,
    /** Signed-in staff id — drives the "My jobs" filter and "assign to me". */
    val userId: String? = null,
    val roles: List<String> = emptyList(),
    val lastPhone: String? = null,
) {
    /** Charging needs both a branch and a stock location to post against. */
    val tillReady: Boolean get() = !branchId.isNullOrBlank() && !locationId.isNullOrBlank()

    /** Reconcile is owner/manager-gated server-side; used to soften the UI copy. */
    val canForceReconcile: Boolean get() = roles.any { it == "owner" || it == "manager" }

    /**
     * Staff vouch for handover without pickup code / OTP — matches
     * `repairs.release_unverified` (owner/manager convention on POS).
     */
    val canReleaseUnverified: Boolean get() = roles.any { it == "owner" || it == "manager" }

    /**
     * Whether Sale Details should show cost/margin. The real gate is the
     * server's reports.read permission check (internal/sales/handler.go) —
     * a cashier-role caller simply never receives unit_cost/margin on the
     * wire. This is UI-side politeness matching that same role convention
     * (Android only tracks roles, not the fine-grained permission list), not
     * an independent security boundary.
     */
    val canSeeCost: Boolean get() = roles.any { it == "owner" || it == "manager" }

    /**
     * Whether to offer intake. Intake is permissioned server-side regardless,
     * so this only avoids showing a button that would 403.
     *
     * Deliberately fails *open* on an empty role list. Roles are only written
     * when `GET /me` succeeds, and that call is best-effort — a slow network at
     * sign-in leaves this handset with no roles at all. Failing closed there
     * silently hides the Jobs screen's primary action with nothing on screen to
     * explain it, which is far worse than letting the server refuse a tap.
     */
    val canCreateIntake: Boolean
        get() = roles.isEmpty() ||
            roles.any { it == "owner" || it == "manager" || it == "cashier" || it == "technician" }
}
