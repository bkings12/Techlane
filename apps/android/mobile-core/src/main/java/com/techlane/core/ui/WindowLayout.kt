package com.techlane.core.ui

import android.content.res.Configuration
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.platform.LocalConfiguration
import com.techlane.core.display.ProvideDisplayCompat

/**
 * Viewport-aware chrome sizing. Short phones keep compact headers but still
 * use the bottom tab bar — a side rail only appears on landscape / tablets.
 */
data class WindowLayout(
    val screenWidthDp: Int,
    val screenHeightDp: Int,
    val isLandscape: Boolean,
    /** Short enough that phone-sized heroes need tighter spacing. */
    val compactChrome: Boolean,
    /** Prefer a left rail instead of a bottom navigation bar. */
    val useSideNav: Boolean,
)

val LocalWindowLayout = staticCompositionLocalOf {
    WindowLayout(
        screenWidthDp = 411,
        screenHeightDp = 800,
        isLandscape = false,
        compactChrome = false,
        useSideNav = false,
    )
}

@Composable
fun rememberWindowLayout(): WindowLayout {
    val configuration = LocalConfiguration.current
    return remember(
        configuration.screenWidthDp,
        configuration.screenHeightDp,
        configuration.orientation,
    ) {
        val width = configuration.screenWidthDp
        val height = configuration.screenHeightDp
        val landscape = configuration.orientation == Configuration.ORIENTATION_LANDSCAPE ||
            (width > 0 && height > 0 && width > height)
        val compact = height in 1..640 || (landscape && height in 1..720)
        // Bottom tabs on phones (including short POS portrait). Side rail only
        // when there is real horizontal room — landscape or tablet-width.
        val sideNav = landscape || width >= 840
        WindowLayout(
            screenWidthDp = width,
            screenHeightDp = height,
            isLandscape = landscape,
            compactChrome = compact,
            useSideNav = sideNav,
        )
    }
}

@Composable
fun ProvideWindowLayout(content: @Composable () -> Unit) {
    ProvideDisplayCompat {
        val layout = rememberWindowLayout()
        CompositionLocalProvider(LocalWindowLayout provides layout, content = content)
    }
}
