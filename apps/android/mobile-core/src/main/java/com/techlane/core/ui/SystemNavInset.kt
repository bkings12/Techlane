package com.techlane.core.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.max

/**
 * Bottom padding that clears the system navigation bar (3-button or gesture).
 *
 * Prefer live [WindowInsets.navigationBars]; if those report 0 (broken OEM /
 * context quirk), fall back to the platform dimen or 48.dp so tab icons never
 * sit under Recent/Home/Back.
 */
@Composable
fun systemNavBottomInset(): Dp {
    val inset = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    if (inset > 0.dp) return inset

    val density = LocalDensity.current
    val context = LocalContext.current
    val fallback = remember(context.resources.configuration.densityDpi, density.density) {
        val id = context.resources.getIdentifier("navigation_bar_height", "dimen", "android")
        if (id > 0) {
            with(density) { context.resources.getDimensionPixelSize(id).toDp() }
        } else {
            48.dp
        }
    }
    return max(fallback, 48.dp)
}

@Composable
fun SystemNavBottomPad(
    color: Color = Color.White,
    modifier: Modifier = Modifier,
) {
    Spacer(
        modifier = modifier
            .fillMaxWidth()
            .height(systemNavBottomInset())
            .background(color),
    )
}

/**
 * Wraps a Material bottom [androidx.compose.material3.NavigationBar] so icons
 * sit above the system nav, with a matching fill behind the system buttons.
 */
@Composable
fun SafeBottomBar(
    containerColor: Color = Color.White,
    content: @Composable () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(containerColor),
    ) {
        content()
        SystemNavBottomPad(color = containerColor)
    }
}
