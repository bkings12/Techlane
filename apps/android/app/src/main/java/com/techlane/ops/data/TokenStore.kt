package com.techlane.ops.data

import android.content.Context
import com.techlane.core.data.SecureTokenStore

/** Staff app token store — keeps the existing prefs name for upgrade continuity. */
class TokenStore(context: Context) {
    private val store = SecureTokenStore(context, "techlane_secure")

    var accessToken: String?
        get() = store.accessToken
        set(value) { store.accessToken = value }

    var refreshToken: String?
        get() = store.refreshToken
        set(value) { store.refreshToken = value }

    var selectedBranchId: String?
        get() = store.selectedBranchId
        set(value) { store.selectedBranchId = value }

    var deviceId: String
        get() = store.deviceId
        set(value) { store.deviceId = value }

    fun clear() = store.clear()
}
