package com.techlane.customer

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.techlane.core.theme.TechLaneTheme
import com.techlane.core.ui.ProvideWindowLayout
import com.techlane.core.update.AppUpdateGate
import com.techlane.customer.ui.CustomerNav

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
                        appKey = "customer",
                        currentVersionCode = BuildConfig.VERSION_CODE,
                    ) {
                        var signedIn by remember {
                            mutableStateOf(CustomerApp.instance.tokenStore.sessionToken != null)
                        }
                        Scaffold(modifier = Modifier.fillMaxSize()) { inner ->
                            CustomerNav(
                                signedIn = signedIn,
                                onSignedIn = { signedIn = true },
                                onSignedOut = {
                                    CustomerApp.instance.tokenStore.clearSession()
                                    signedIn = false
                                },
                                // BrandHero / BrandDetailHeader own the status-bar inset.
                                rootModifier = Modifier.padding(bottom = inner.calculateBottomPadding()),
                            )
                        }
                    }
                }
            }
        }
    }
}
