package com.techlane.core.display

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Density
import android.content.res.Configuration

/**
 * Normalizes Compose density for POS terminals and other short/odd screens.
 *
 * Applied only via [ProvideDisplayCompat] — never via Activity.attachBaseContext.
 * Wrapping the Activity with createConfigurationContext breaks WindowInsets
 * delivery, which leaves the app tab bar under the system navigation buttons.
 */
object DisplayCompat {
    private const val MIN_PORTRAIT_HEIGHT_DP = 640
    private const val MIN_LANDSCAPE_HEIGHT_DP = 560

    /**
     * Density multiplier relative to the platform density. Values below 1.0
     * shrink UI so more content fits on short panels.
     */
    fun densityScale(
        screenHeightDp: Int,
        orientation: Int,
        widthDp: Int = 0,
        heightDp: Int = screenHeightDp,
    ): Float {
        val landscape = orientation == Configuration.ORIENTATION_LANDSCAPE ||
            (widthDp > 0 && heightDp > 0 && widthDp > heightDp)
        val targetHeightDp = if (landscape) MIN_LANDSCAPE_HEIGHT_DP else MIN_PORTRAIT_HEIGHT_DP
        if (screenHeightDp !in 1 until targetHeightDp) return 1f
        // Shrink so the short panel behaves like targetHeightDp tall.
        val scale = screenHeightDp / targetHeightDp.toFloat()
        return scale.coerceIn(0.55f, 1f)
    }
}

/**
 * Adjusts [LocalDensity] for short POS panels and caps fontScale at 1f.
 * Does not touch the Activity context, so system bar insets keep working.
 */
@Composable
fun ProvideDisplayCompat(content: @Composable () -> Unit) {
    val base = LocalDensity.current
    val configuration = LocalConfiguration.current
    val adjusted = remember(
        configuration.screenWidthDp,
        configuration.screenHeightDp,
        configuration.orientation,
        base.density,
        base.fontScale,
    ) {
        val scale = DisplayCompat.densityScale(
            screenHeightDp = configuration.screenHeightDp,
            orientation = configuration.orientation,
            widthDp = configuration.screenWidthDp,
            heightDp = configuration.screenHeightDp,
        )
        Density(
            density = base.density * scale,
            fontScale = 1f,
        )
    }
    CompositionLocalProvider(LocalDensity provides adjusted, content = content)
}
