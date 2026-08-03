package com.techlane.pos.core.designsystem.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

/** App-wide appearance preference, persisted in DataStore. */
enum class ThemeMode { SYSTEM, LIGHT, DARK }

@Composable
fun TechLanePosTheme(
    themeMode: ThemeMode = ThemeMode.SYSTEM,
    content: @Composable () -> Unit,
) {
    val dark = when (themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }
    // Deliberately not using dynamic color: a shop-floor tool should look the same
    // on every technician's phone, and the brand navy/gold is part of the product.
    val colorScheme = if (dark) TlDarkColorScheme else TlLightColorScheme
    val semantic = if (dark) DarkSemanticColors else LightSemanticColors

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as? Activity)?.window ?: return@SideEffect
            WindowCompat.setDecorFitsSystemWindows(window, false)
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = !dark
                isAppearanceLightNavigationBars = !dark
            }
        }
    }

    CompositionLocalProvider(
        LocalTlSemanticColors provides semantic,
        LocalTlSpacing provides TlSpacing(),
        LocalTlSizes provides TlSizes(),
    ) {
        MaterialTheme(
            colorScheme = colorScheme,
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
internal fun TlPreview(dark: Boolean = false, content: @Composable () -> Unit) {
    LocalContext.current
    TechLanePosTheme(themeMode = if (dark) ThemeMode.DARK else ThemeMode.LIGHT, content = content)
}
