package com.techlane.pos.core.designsystem.theme

import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * TechLane brand values. Screens never reach for these — they read
 * [androidx.compose.material3.MaterialTheme.colorScheme] or [TlTheme.colors]
 * instead, so a retint only ever happens here.
 *
 * Navy anchors headings and totals; Blue carries every primary action; Bright
 * is the pressed/active step above it; Amber is an accent for "needs
 * attention" and nothing else.
 */
internal object BrandPalette {
    val Navy = Color(0xFF102A43)
    val Blue = Color(0xFF1565C0)
    val BlueBright = Color(0xFF2583E8)
    val BlueTint = Color(0xFFEEF6FF)
    val BlueTintStrong = Color(0xFFD6E9FC)

    val Amber = Color(0xFFF59E0B)
    val AmberTint = Color(0xFFFFFBEB)
    val AmberDeep = Color(0xFF92400E)

    /** Body copy. Near-black with a cool cast, never pure #000. */
    val Ink = Color(0xFF172033)
    val TextSecondary = Color(0xFF667085)
    val TextMuted = Color(0xFF98A2B3)

    val Border = Color(0xFFE2E8F0)
    val BorderStrong = Color(0xFFCBD5E1)
    val SurfaceSecondary = Color(0xFFF8FAFC)
    val Canvas = Color(0xFFF6F8FB)

    val Success = Color(0xFF16A34A)
    val SuccessTint = Color(0xFFF0FDF4)
    val SuccessDeep = Color(0xFF14532D)
    val Danger = Color(0xFFDC2626)
    val DangerTint = Color(0xFFFEF2F2)
    val DangerDeep = Color(0xFF7F1D1D)
    /** Diagnosed/in-review states, kept distinct from the brand blue. */
    val Violet = Color(0xFF7C3AED)
    val VioletTint = Color(0xFFF5F3FF)
    val VioletDeep = Color(0xFF4C1D95)
}

val TlLightColorScheme = lightColorScheme(
    // Blue, not Navy, is primary: it is the action colour, and Navy is dark
    // enough that a screen full of it reads as black rather than as a brand.
    primary = BrandPalette.Blue,
    onPrimary = Color.White,
    primaryContainer = BrandPalette.BlueTint,
    onPrimaryContainer = BrandPalette.Blue,
    inversePrimary = BrandPalette.BlueBright,
    secondary = BrandPalette.Navy,
    onSecondary = Color.White,
    secondaryContainer = BrandPalette.BlueTintStrong,
    onSecondaryContainer = BrandPalette.Navy,
    tertiary = BrandPalette.Amber,
    onTertiary = Color.White,
    tertiaryContainer = BrandPalette.AmberTint,
    onTertiaryContainer = BrandPalette.AmberDeep,
    background = BrandPalette.Canvas,
    onBackground = BrandPalette.Navy,
    surface = Color.White,
    onSurface = BrandPalette.Ink,
    surfaceVariant = BrandPalette.SurfaceSecondary,
    // Secondary text is a readable slate, not the near-invisible grey Material
    // defaults to — this single slot drives most of the app's body copy.
    onSurfaceVariant = BrandPalette.TextSecondary,
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color.White,
    surfaceContainer = BrandPalette.SurfaceSecondary,
    surfaceContainerHigh = BrandPalette.Canvas,
    surfaceContainerHighest = BrandPalette.BlueTint,
    inverseSurface = BrandPalette.Navy,
    inverseOnSurface = Color.White,
    outline = BrandPalette.BorderStrong,
    outlineVariant = BrandPalette.Border,
    error = BrandPalette.Danger,
    onError = Color.White,
    errorContainer = BrandPalette.DangerTint,
    onErrorContainer = BrandPalette.DangerDeep,
    scrim = BrandPalette.Navy,
)

/**
 * Semantics Material 3 has no slot for: pass/fail/waiting states, money
 * emphasis, and the hairline used on cards. Read through [TlTheme.colors].
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
    val violet: Color,
    val violetContainer: Color,
    val onVioletContainer: Color,
    val hairline: Color,
    val elevatedSurface: Color,
    /** Navy — headings, totals, and anything that should read as "TechLane". */
    val brand: Color,
    val brandBright: Color,
    val moneyPositive: Color,
    val scrimHeavy: Color,
)

val LightSemanticColors = TlSemanticColors(
    success = BrandPalette.Success,
    onSuccess = Color.White,
    successContainer = BrandPalette.SuccessTint,
    onSuccessContainer = BrandPalette.SuccessDeep,
    warning = BrandPalette.Amber,
    onWarning = Color.White,
    warningContainer = BrandPalette.AmberTint,
    onWarningContainer = BrandPalette.AmberDeep,
    info = BrandPalette.Blue,
    infoContainer = BrandPalette.BlueTint,
    onInfoContainer = BrandPalette.Blue,
    accent = BrandPalette.Amber,
    onAccent = Color.White,
    violet = BrandPalette.Violet,
    violetContainer = BrandPalette.VioletTint,
    onVioletContainer = BrandPalette.VioletDeep,
    hairline = BrandPalette.Border,
    elevatedSurface = Color.White,
    brand = BrandPalette.Navy,
    brandBright = BrandPalette.BlueBright,
    moneyPositive = BrandPalette.Success,
    scrimHeavy = Color(0xE6102A43),
)

internal val LocalTlSemanticColors = staticCompositionLocalOf { LightSemanticColors }
