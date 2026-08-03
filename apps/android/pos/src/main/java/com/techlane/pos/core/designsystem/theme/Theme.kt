package com.techlane.pos.core.designsystem.theme

import android.app.Activity
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

/**
 * TechLane Light Mode — the one deterministic appearance for every technician
 * on every phone. No dark theme, no dynamic/wallpaper-driven color: a
 * shop-floor tool should look identical everywhere, and the brand navy/gold
 * is part of the product, not something the OS gets to override.
 */
@Composable
fun TechLanePosTheme(content: @Composable () -> Unit) {
    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as? Activity)?.window ?: return@SideEffect
            WindowCompat.setDecorFitsSystemWindows(window, false)
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = true
                isAppearanceLightNavigationBars = true
            }
        }
    }

    CompositionLocalProvider(
        LocalTlSemanticColors provides LightSemanticColors,
        LocalTlSpacing provides TlSpacing(),
        LocalTlSizes provides TlSizes(),
    ) {
        MaterialTheme(
            colorScheme = TlLightColorScheme,
            typography = TlTypography,
            shapes = TlShapes,
            content = content,
        )
    }
}

/** Accessors for the tokens Material 3 does not carry. */
object TlTheme {
    val colors: TlSemanticColors
        @Composable @ReadOnlyComposable get() = LocalTlSemanticColors.current

    val spacing: TlSpacing
        @Composable @ReadOnlyComposable get() = LocalTlSpacing.current

    val sizes: TlSizes
        @Composable @ReadOnlyComposable get() = LocalTlSizes.current

    val typography: Typography
        @Composable @ReadOnlyComposable get() = MaterialTheme.typography
}

/** Convenience for previews so component files stay free of boilerplate. */
@Composable
internal fun TlPreview(content: @Composable () -> Unit) {
    LocalContext.current
    TechLanePosTheme(content = content)
}
