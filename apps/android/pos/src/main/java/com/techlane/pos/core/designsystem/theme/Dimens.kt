package com.techlane.pos.core.designsystem.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Shapes
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** 4dp base scale. Screens use these instead of inventing one-off paddings. */
@Immutable
data class TlSpacing(
    val xxs: Dp = 2.dp,
    val xs: Dp = 4.dp,
    val sm: Dp = 8.dp,
    val md: Dp = 12.dp,
    val lg: Dp = 16.dp,
    val xl: Dp = 20.dp,
    val xxl: Dp = 24.dp,
    val xxxl: Dp = 32.dp,
    val huge: Dp = 48.dp,
    /** Horizontal page gutter — widens on tablets via TlWindow. */
    val gutter: Dp = 20.dp,
)

/** Touch targets sized for a technician working fast, often one-handed. */
@Immutable
data class TlSizes(
    val controlHeight: Dp = 56.dp,
    val controlHeightLarge: Dp = 64.dp,
    val controlHeightSmall: Dp = 44.dp,
    val minTouchTarget: Dp = 48.dp,
    val iconSm: Dp = 18.dp,
    val icon: Dp = 22.dp,
    val iconLg: Dp = 28.dp,
    val avatar: Dp = 40.dp,
    val bottomBarHeight: Dp = 72.dp,
)

val TlShapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(12.dp),
    medium = RoundedCornerShape(16.dp),
    large = RoundedCornerShape(22.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

val PillShape = RoundedCornerShape(999.dp)

internal val LocalTlSpacing = staticCompositionLocalOf { TlSpacing() }
internal val LocalTlSizes = staticCompositionLocalOf { TlSizes() }
