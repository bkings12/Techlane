package com.techlane.core.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

// Workshop navy + service blue: calm neutrals with semantic status color.
private val Ink = Color(0xFF0B1220)
private val Slate = Color(0xFF111A2B)
private val Panel = Color(0xFF182337)
private val Mist = Color(0xFFF3F6FA)
private val Paper = Color(0xFFFFFFFF)
private val Blue = Color(0xFF155EEF)
private val BlueBright = Color(0xFF84ADFF)
private val Cyan = Color(0xFF0891B2)
private val InkText = Color(0xFF101828)
private val MistText = Color(0xFFF2F4F7)

private val DarkColors = darkColorScheme(
    primary = BlueBright,
    onPrimary = Ink,
    primaryContainer = Color(0xFF102A56),
    onPrimaryContainer = Color(0xFFD1E0FF),
    secondary = Color(0xFF67E3F9),
    onSecondary = Ink,
    secondaryContainer = Color(0xFF164E63),
    onSecondaryContainer = Color(0xFFCFFAFE),
    tertiary = Color(0xFFFDB022),
    background = Ink,
    onBackground = MistText,
    surface = Slate,
    onSurface = MistText,
    surfaceVariant = Panel,
    onSurfaceVariant = Color(0xFFB8C4D4),
    surfaceContainerLowest = Color(0xFF080E19),
    surfaceContainerLow = Slate,
    surfaceContainer = Panel,
    surfaceContainerHigh = Color(0xFF202C42),
    surfaceContainerHighest = Color(0xFF29364B),
    outline = Color(0xFF3D4A5C),
    error = Color(0xFFF87171),
    onError = Ink,
)

private val LightColors = lightColorScheme(
    primary = Blue,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFD1E0FF),
    onPrimaryContainer = Color(0xFF102A56),
    secondary = Color(0xFF0E7490),
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFCFFAFE),
    onSecondaryContainer = Color(0xFF164E63),
    tertiary = Color(0xFFB54708),
    background = Mist,
    onBackground = InkText,
    surface = Paper,
    onSurface = InkText,
    surfaceVariant = Color(0xFFE4E7EC),
    onSurfaceVariant = Color(0xFF475467),
    surfaceContainerLowest = Paper,
    surfaceContainerLow = Color(0xFFF9FAFB),
    surfaceContainer = Color(0xFFF2F4F7),
    surfaceContainerHigh = Color(0xFFEAECF0),
    surfaceContainerHighest = Color(0xFFE4E7EC),
    outline = Color(0xFF98A2B3),
    error = Color(0xFFDC2626),
    onError = Color.White,
)

private val TechLaneShapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp),
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp),
    extraLarge = RoundedCornerShape(24.dp),
)

@Composable
fun TechLaneTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    MaterialTheme(
        colorScheme = if (dark) DarkColors else LightColors,
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
    intake = Color(0xFF155EEF),
    diagnosed = Color(0xFF0E7490),
    waitingParts = Color(0xFFB54708),
    inProgress = Color(0xFF047857),
    completed = Color(0xFF0F766E),
    collected = Color(0xFF475467),
)

private val DarkStatus = StatusPalette(
    intake = Color(0xFF84ADFF),
    diagnosed = Color(0xFF67E3F9),
    waitingParts = Color(0xFFFDB022),
    inProgress = Color(0xFF34D399),
    completed = Color(0xFF2DD4BF),
    collected = Color(0xFF98A2B3),
)

@Composable
fun statusPalette(): StatusPalette = if (isSystemInDarkTheme()) DarkStatus else LightStatus

val StatusIntake = LightStatus.intake
val StatusDiagnosed = LightStatus.diagnosed
val StatusWaitingParts = LightStatus.waitingParts
val StatusInProgress = LightStatus.inProgress
val StatusCompleted = LightStatus.completed
val StatusCollected = LightStatus.collected
val StatusSuccess = LightStatus.inProgress
val StatusWarning = LightStatus.waitingParts
val StatusDanger = Color(0xFFDC2626)
val ElevatedSurface = Panel
