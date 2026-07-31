package com.techlane.supplier

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.techlane.core.theme.TechLaneTheme
import com.techlane.core.ui.ProvideWindowLayout
import com.techlane.core.update.AppUpdateGate
import com.techlane.supplier.ui.SupplierNav

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.light(
                android.graphics.Color.WHITE,
                android.graphics.Color.WHITE,
            ),
        )
        setContent {
            TechLaneTheme {
                ProvideWindowLayout {
                    AppUpdateGate(
                        apiBase = BuildConfig.API_BASE,
                        appKey = "supplier",
                        currentVersionCode = BuildConfig.VERSION_CODE,
                    ) {
                        var signedIn by remember {
                            mutableStateOf(SupplierApp.instance.tokenStore.sessionToken != null)
                        }
                        SupplierNav(
                            signedIn = signedIn,
                            onSignedIn = { signedIn = true },
                            onSignedOut = {
                                SupplierApp.instance.tokenStore.clearSession()
                                signedIn = false
                            },
                            rootModifier = Modifier.fillMaxSize(),
                        )
                    }
                }
            }
        }
    }
}
