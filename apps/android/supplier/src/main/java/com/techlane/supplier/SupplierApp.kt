package com.techlane.supplier

import android.app.Application
import com.techlane.core.data.SecureTokenStore

class SupplierApp : Application() {
    lateinit var tokenStore: SecureTokenStore
        private set

    override fun onCreate() {
        super.onCreate()
        instance = this
        tokenStore = SecureTokenStore(this, "techlane_supplier_secure")
    }

    companion object {
        lateinit var instance: SupplierApp
            private set
    }
}
