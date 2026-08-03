package com.techlane.pos

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.lifecycleScope
import com.techlane.core.display.ProvideDisplayCompat
import com.techlane.pos.core.designsystem.theme.TechLanePosTheme
import com.techlane.pos.data.remote.SessionExpiryNotifier
import com.techlane.pos.data.repository.AuthRepository
import com.techlane.pos.data.session.SecureTokenStore
import com.techlane.pos.navigation.PosApp
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * FragmentActivity (not plain ComponentActivity) because BiometricPrompt needs a
 * fragment host to attach its system dialog to.
 */
@AndroidEntryPoint
class MainActivity : FragmentActivity() {

    @Inject lateinit var tokens: SecureTokenStore
    @Inject lateinit var sessionExpiry: SessionExpiryNotifier
    @Inject lateinit var auth: AuthRepository

    override fun onCreate(savedInstanceState: Bundle?) {
        installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        // A 401 that survives a token refresh means the session is truly over.
        lifecycleScope.launch {
            sessionExpiry.expired.collect { tokens.clear() }
        }

        // Re-read the profile on every launch. Roles are otherwise only written
        // when GET /me happens to succeed at sign-in, so a handset that signed
        // in on a bad connection would carry an empty role list — and the
        // permissions it gates — until someone signed out and back in.
        lifecycleScope.launch {
            if (tokens.refreshToken != null) auth.refreshProfile()
        }

        setContent {
            val signedIn by tokens.signedIn.collectAsState()

            TechLanePosTheme {
                // Shrinks density and caps fontScale at 1x on short POS-terminal
                // panels (e.g. PDQ card machines) — same fix Ops/Customer/Supplier
                // already apply via :mobile-core, previously missing here.
                ProvideDisplayCompat {
                    PosApp(signedIn = signedIn)
                }
            }
        }
    }
}
