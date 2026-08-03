package com.techlane.pos.core.designsystem.theme

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * Raw brand values from design-tokens/tokens.json. Screens never reach for these —
 * they read [androidx.compose.material3.MaterialTheme.colorScheme] or [TlTheme.colors]
 * so light and dark stay in step.
 */
internal object BrandPalette {
    val Navy = Color(0xFF060386)
    val NavyDark = Color(0xFF040257)
    val NavyDeep = Color(0xFF02012E)
    val NavyTint = Color(0xFFE6E5FA)
    val NavyLight = Color(0xFFA9A7F0)

    val Gold = Color(0xFFF2BE2A)
    val GoldDark = Color(0xFFC69C22)
    val GoldTint = Color(0xFFFCF0CD)

    val Ink = Color(0xFF0F172A)
    val Slate700 = Color(0xFF334155)
    val Slate600 = Color(0xFF475569)
    val Slate500 = Color(0xFF64748B)
    val Slate300 = Color(0xFFCBD5E1)
    val Slate200 = Color(0xFFE2E8F0)
    val Slate100 = Color(0xFFEEF2F7)
    val Canvas = Color(0xFFF5F7FB)

    val NightCanvas = Color(0xFF090D18)
    val NightSurface = Color(0xFF121728)
    val NightSurfaceHigh = Color(0xFF1A2135)
    val NightSurfaceHighest = Color(0xFF232B42)
    val NightBorder = Color(0xFF2C3550)
    val NightText = Color(0xFFE8ECF5)
    val NightTextMuted = Color(0xFF9AA5BD)

    val Success = Color(0xFF059669)
    val SuccessDark = Color(0xFF34D399)
    val Warning = Color(0xFFD97706)
    val WarningDark = Color(0xFFFBBF24)
    val Danger = Color(0xFFDC2626)
    val DangerDark = Color(0xFFF87171)
    val Info = Color(0xFF4F46E5)
    val InfoDark = Color(0xFF9B95F5)
}

val TlLightColorScheme = lightColorScheme(
    primary = BrandPalette.Navy,
    onPrimary = Color.White,
    primaryContainer = BrandPalette.NavyTint,
    onPrimaryContainer = BrandPalette.NavyDark,
    inversePrimary = BrandPalette.NavyLight,
    secondary = BrandPalette.GoldDark,
    onSecondary = Color(0xFF1D1704),
    secondaryContainer = BrandPalette.GoldTint,
    onSecondaryContainer = Color(0xFF5C470A),
    tertiary = BrandPalette.Gold,
    onTertiary = Color(0xFF1D1704),
    tertiaryContainer = BrandPalette.GoldTint,
    onTertiaryContainer = Color(0xFF5C470A),
    background = BrandPalette.Canvas,
    onBackground = BrandPalette.Ink,
    surface = Color.White,
    onSurface = BrandPalette.Ink,
    surfaceVariant = BrandPalette.Slate100,
    onSurfaceVariant = BrandPalette.Slate600,
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color(0xFFFBFCFE),
    surfaceContainer = BrandPalette.Canvas,
    surfaceContainerHigh = BrandPalette.Slate100,
    surfaceContainerHighest = Color(0xFFE5EAF1),
    inverseSurface = Color(0xFF111A2B),
    inverseOnSurface = Color(0xFFF2F4F7),
    outline = BrandPalette.Slate300,
    outlineVariant = BrandPalette.Slate200,
    error = BrandPalette.Danger,
    onError = Color.White,
    errorContainer = Color(0xFFFEE2E2),
    onErrorContainer = Color(0xFF7F1D1D),
    scrim = Color(0xFF060B18),
)

val TlDarkColorScheme = darkColorScheme(
    primary = BrandPalette.NavyLight,
    onPrimary = BrandPalette.NavyDeep,
    primaryContainer = Color(0xFF231F8C),
    onPrimaryContainer = Color(0xFFDDDCFB),
    inversePrimary = BrandPalette.Navy,
    secondary = BrandPalette.Gold,
    onSecondary = Color(0xFF1D1704),
    secondaryContainer = Color(0xFF473509),
    onSecondaryContainer = BrandPalette.GoldTint,
    tertiary = BrandPalette.Gold,
    onTertiary = Color(0xFF1D1704),
    tertiaryContainer = Color(0xFF473509),
    onTertiaryContainer = BrandPalette.GoldTint,
    background = BrandPalette.NightCanvas,
    onBackground = BrandPalette.NightText,
    surface = BrandPalette.NightSurface,
    onSurface = BrandPalette.NightText,
    surfaceVariant = BrandPalette.NightSurfaceHigh,
    onSurfaceVariant = BrandPalette.NightTextMuted,
    surfaceContainerLowest = Color(0xFF0B101C),
    surfaceContainerLow = Color(0xFF0F1422),
    surfaceContainer = BrandPalette.NightSurface,
    surfaceContainerHigh = BrandPalette.NightSurfaceHigh,
    surfaceContainerHighest = BrandPalette.NightSurfaceHighest,
    inverseSurface = Color(0xFFE8ECF5),
    inverseOnSurface = Color(0xFF111A2B),
    outline = BrandPalette.NightBorder,
    outlineVariant = Color(0xFF222A40),
    error = BrandPalette.DangerDark,
    onError = Color(0xFF450A0A),
    errorContainer = Color(0xFF4C1414),
    onErrorContainer = Color(0xFFFECACA),
    scrim = Color(0xFF01030A),
)

/**
 * Semantics Material 3 has no slot for: pass/fail/waiting states, money emphasis,
 * and the hairline used on cards. Read through [TlTheme.colors].
 */
@Immutable
data class TlSemanticColors(
    val success: Color,
    val onSuccess: Color,
    val successContainer: Color,
    val onSuccessContainer: Color,
    val warning: Color,
    val onWarning: Color,
    val warningContainer: Color,
    val onWarningContainer: Color,
    val info: Color,
    val infoContainer: Color,
    val onInfoContainer: Color,
    val accent: Color,
    val onAccent: Color,
    val hairline: Color,
    val elevatedSurface: Color,
    val moneyPositive: Color,
    val scrimHeavy: Color,
)

val LightSemanticColors = TlSemanticColors(
    success = BrandPalette.Success,
    onSuccess = Color.White,
    successContainer = Color(0xFFD1FAE5),
    onSuccessContainer = Color(0xFF064E3B),
    warning = BrandPalette.Warning,
    onWarning = Color.White,
    warningContainer = Color(0xFFFEF3C7),
    onWarningContainer = Color(0xFF78350F),
    info = BrandPalette.Info,
    infoContainer = Color(0xFFE0E7FF),
    onInfoContainer = Color(0xFF312E81),
    accent = BrandPalette.Gold,
    onAccent = Color(0xFF1D1704),
    hairline = BrandPalette.Slate200,
    elevatedSurface = Color.White,
    moneyPositive = BrandPalette.Success,
    scrimHeavy = Color(0xE60B1220),
)

val DarkSemanticColors = TlSemanticColors(
    success = BrandPalette.SuccessDark,
    onSuccess = Color(0xFF00281B),
    successContainer = Color(0xFF10402F),
    onSuccessContainer = Color(0xFFB9F5DC),
    warning = BrandPalette.WarningDark,
    onWarning = Color(0xFF3A2503),
    warningContainer = Color(0xFF4A3208),
    onWarningContainer = Color(0xFFFDE8B5),
    info = BrandPalette.InfoDark,
    infoContainer = Color(0xFF272364),
    onInfoContainer = Color(0xFFDDDCFB),
    accent = BrandPalette.Gold,
    onAccent = Color(0xFF1D1704),
    hairline = BrandPalette.NightBorder,
    elevatedSurface = BrandPalette.NightSurfaceHigh,
    moneyPositive = BrandPalette.SuccessDark,
    scrimHeavy = Color(0xF20A0E1A),
)

internal val LocalTlSemanticColors = staticCompositionLocalOf { LightSemanticColors }
