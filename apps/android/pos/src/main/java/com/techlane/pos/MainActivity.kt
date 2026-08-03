package com.techlane.pos

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.lifecycleScope
import com.techlane.pos.core.designsystem.theme.TechLanePosTheme
import com.techlane.pos.core.designsystem.theme.ThemeMode
import com.techlane.pos.data.remote.SessionExpiryNotifier
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.data.session.SecureTokenStore
import com.techlane.pos.navigation.PosApp
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * FragmentActivity (not plain ComponentActivity) because BiometricPrompt needs a
 * fragment host to attach its system dialog to.
 */
@AndroidEntryPoint
class MainActivity : FragmentActivity() {

    @Inject lateinit var tokens: SecureTokenStore
    @Inject lateinit var preferences: PreferencesStore
    @Inject lateinit var sessionExpiry: SessionExpiryNotifier

    private var themeResolved = false

    override fun onCreate(savedInstanceState: Bundle?) {
        val splash = installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        // Hold the splash until the stored theme has loaded, so the app never
        // flashes a light screen on its way into dark mode.
        splash.setKeepOnScreenCondition { !themeResolved }

        // A 401 that survives a token refresh means the session is truly over.
        lifecycleScope.launch {
            sessionExpiry.expired.collect { tokens.clear() }
        }

        setContent {
            val themeMode by preferences.preferences
                .map { it.themeMode }
                .collectAsState(initial = ThemeMode.SYSTEM)
            val signedIn by tokens.signedIn.collectAsState()

            LaunchedEffect(Unit) { themeResolved = true }

            TechLanePosTheme(themeMode = themeMode) {
                PosApp(signedIn = signedIn)
            }
        }
    }
}
