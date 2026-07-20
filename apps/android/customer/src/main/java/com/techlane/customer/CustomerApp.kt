package com.techlane.customer

import android.app.Application
import com.techlane.core.data.SecureTokenStore

class CustomerApp : Application() {
    lateinit var tokenStore: SecureTokenStore
        private set

    override fun onCreate() {
        super.onCreate()
        instance = this
        tokenStore = SecureTokenStore(this, "techlane_customer_secure")
    }

    companion object {
        lateinit var instance: CustomerApp
            private set
    }
}
