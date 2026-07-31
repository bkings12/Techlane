package com.techlane.core.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

// TechLane brand tokens (design-tokens/tokens.json). The apps ship light-only:
// deep brand navy for structure and actions, gold reserved for hero accents.
object Brand {
    val Navy = Color(0xFF060386)
    val NavyDark = Color(0xFF040257)
    val NavyDeep = Color(0xFF02012E)
    val Gold = Color(0xFFF2BE2A)
    val GoldDark = Color(0xFFC69C22)
    val GoldTint = Color(0xFFFCF0CD)
    val NavyTint = Color(0xFFE6E5FA)
    val Canvas = Color(0xFFF6F8FC)
    val Surface = Color(0xFFFFFFFF)
    val Subtle = Color(0xFFEEF2F7)
    val TextPrimary = Color(0xFF0F172A)
    val TextSecondary = Color(0xFF475569)
    val TextMuted = Color(0xFF64748B)
    val Border = Color(0xFFE2E8F0)
    val BorderStrong = Color(0xFFCBD5E1)
    val Success = Color(0xFF059669)
    val Warning = Color(0xFFD97706)
    val Danger = Color(0xFFDC2626)
    val Info = Color(0xFF4F46E5)
}

private val LightColors = lightColorScheme(
    primary = Brand.Navy,
    onPrimary = Color.White,
    primaryContainer = Brand.NavyTint,
    onPrimaryContainer = Brand.NavyDark,
    inversePrimary = Color(0xFFB9B7F5),
    secondary = Color(0xFF7A6210),
    onSecondary = Color.White,
    secondaryContainer = Brand.GoldTint,
    onSecondaryContainer = Color(0xFF5C470A),
    tertiary = Brand.Gold,
    onTertiary = Color(0xFF1D1704),
    tertiaryContainer = Brand.GoldTint,
    onTertiaryContainer = Color(0xFF5C470A),
    background = Brand.Canvas,
    onBackground = Brand.TextPrimary,
    surface = Brand.Surface,
    onSurface = Brand.TextPrimary,
    surfaceVariant = Brand.Subtle,
    onSurfaceVariant = Brand.TextSecondary,
    surfaceTint = Color.White,
    inverseSurface = Color(0xFF111A2B),
    inverseOnSurface = Color(0xFFF2F4F7),
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color(0xFFFBFCFE),
    surfaceContainer = Brand.Canvas,
    surfaceContainerHigh = Brand.Subtle,
    surfaceContainerHighest = Color(0xFFE5EAF1),
    outline = Brand.BorderStrong,
    outlineVariant = Brand.Border,
    error = Brand.Danger,
    onError = Color.White,
    errorContainer = Color(0xFFFEE2E2),
    onErrorContainer = Color(0xFF7F1D1D),
)

private val TechLaneShapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(10.dp),
    medium = RoundedCornerShape(14.dp),
    large = RoundedCornerShape(20.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

@Composable
fun TechLaneTheme(content: @Composable () -> Unit) {
    // Brand decision: light mode only across all TechLane apps.
    MaterialTheme(
        colorScheme = LightColors,
        typography = TechLaneTypography,
        shapes = TechLaneShapes,
        content = content,
    )
}

data class StatusPalette(
    val intake: Color,
    val diagnosed: Color,
    val waitingParts: Color,
    val inProgress: Color,
    val completed: Color,
    val collected: Color,
)

private val LightStatus = StatusPalette(
    intake = Brand.Info,
    diagnosed = Color(0xFF0E7490),
    waitingParts = Brand.Warning,
    inProgress = Color(0xFF047857),
    completed = Color(0xFF0F766E),
    collected = Brand.TextMuted,
)

@Composable
fun statusPalette(): StatusPalette = LightStatus

val StatusSuccess = Brand.Success
val StatusWarning = Brand.Warning
val StatusDanger = Brand.Danger
