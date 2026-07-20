package com.techlane.ops

import android.os.Bundle
import androidx.activity.ComponentActivity
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
import com.techlane.ops.ui.AppNav
import com.techlane.core.theme.TechLaneTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            TechLaneTheme {
                var signedIn by remember {
                    mutableStateOf(TechLaneApp.instance.tokenStore.accessToken != null)
                }
                Scaffold(modifier = Modifier.fillMaxSize()) { inner ->
                    AppNav(
                        signedIn = signedIn,
                        onSignedIn = { signedIn = true },
                        onSignedOut = {
                            TechLaneApp.instance.tokenStore.clear()
                            signedIn = false
                        },
                        modifier = Modifier.padding(inner),
                    )
                }
            }
        }
    }
}
