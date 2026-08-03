package com.techlane.pos.core.designsystem.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.LineHeightStyle
import androidx.compose.ui.unit.sp
import com.techlane.pos.R

/** Outfit is the TechLane brand face; it ships as a single variable-weight file. */
private val Outfit = FontFamily(
    Font(R.font.outfit, FontWeight.Light),
    Font(R.font.outfit, FontWeight.Normal),
    Font(R.font.outfit, FontWeight.Medium),
    Font(R.font.outfit, FontWeight.SemiBold),
    Font(R.font.outfit, FontWeight.Bold),
)

private val TightLineHeight = LineHeightStyle(
    alignment = LineHeightStyle.Alignment.Center,
    trim = LineHeightStyle.Trim.None,
)

private fun tl(
    size: Int,
    lineHeight: Int,
    weight: FontWeight = FontWeight.Normal,
    tracking: Double = 0.0,
) = TextStyle(
    fontFamily = Outfit,
    fontWeight = weight,
    fontSize = size.sp,
    lineHeight = lineHeight.sp,
    letterSpacing = tracking.sp,
    lineHeightStyle = TightLineHeight,
    platformStyle = PlatformTextStyle(includeFontPadding = false),
)

val TlTypography = Typography(
    displayLarge = tl(56, 60, FontWeight.Bold, -1.0),
    displayMedium = tl(45, 50, FontWeight.Bold, -0.8),
    displaySmall = tl(36, 42, FontWeight.SemiBold, -0.5),
    headlineLarge = tl(32, 38, FontWeight.SemiBold, -0.4),
    headlineMedium = tl(28, 34, FontWeight.SemiBold, -0.3),
    headlineSmall = tl(24, 30, FontWeight.SemiBold, -0.2),
    titleLarge = tl(21, 27, FontWeight.SemiBold, -0.1),
    titleMedium = tl(17, 23, FontWeight.SemiBold),
    titleSmall = tl(15, 20, FontWeight.SemiBold),
    bodyLarge = tl(16, 24),
    bodyMedium = tl(14, 21),
    bodySmall = tl(13, 19),
    labelLarge = tl(15, 20, FontWeight.SemiBold, 0.1),
    labelMedium = tl(13, 17, FontWeight.Medium, 0.2),
    labelSmall = tl(11, 15, FontWeight.SemiBold, 0.5),
)

/** Tabular-ish numeral style for money — keeps totals from jittering as they update. */
val MoneyDisplay = tl(44, 50, FontWeight.Bold, -1.2)
val MoneyTitle = tl(24, 30, FontWeight.Bold, -0.4)
